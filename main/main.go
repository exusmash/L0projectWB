package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/nats-io/stan.go"
	"html/template"
	"log"
	"net/http"
	"time"
)

type Delivery struct {
	Name    string `json:"name" db:"name"`
	Phone   string `json:"phone" db:"phone"`
	Zip     string `json:"zip" db:"zip"`
	City    string `json:"city" db:"city"`
	Address string `json:"address" db:"address"`
	Region  string `json:"region" db:"region"`
	Email   string `json:"email" db:"email"`
}

type Payment struct {
	Transaction  string `json:"transaction" db:"transaction"`
	RequestId    string `json:"request_id" db:"request_id"`
	Currency     string `json:"currency" db:"currency"`
	Provider     string `json:"provider" db:"provider"`
	Amount       int    `json:"amount" db:"amount"`
	PaymentDt    int    `json:"payment_dt" db:"payment_dt"`
	Bank         string `json:"bank" db:"bank"`
	DeliveryCost int    `json:"delivery_cost" db:"delivery_cost"`
	GoodsTotal   int    `json:"goods_total" db:"goods_total"`
	CustomFee    int    `json:"custom_fee" db:"custom_fee"`
}

type Item struct {
	ChrtId      int    `json:"chrt_id" db:"chrt_id"`
	TrackNumber string `json:"track_number" db:"track_number"`
	Price       int    `json:"price" db:"price"`
	Rid         string `json:"rid" db:"rid"`
	Name        string `json:"name" db:"name"`
	Sale        int    `json:"sale" db:"sale"`
	Size        string `json:"size" db:"size"`
	TotalPrice  int    `json:"total_price" db:"total_price"`
	NmId        int    `json:"nm_id" db:"nm_id"`
	Brand       string `json:"brand" db:"brand"`
	Status      int    `json:"status" db:"status"`
}

type Order struct {
	OrderUid          string    `json:"order_uid" db:"order_uid"`
	TrackNumber       string    `json:"track_number" db:"track_number"`
	Entry             string    `json:"entry" db:"entry"`
	Delivery          Delivery  `json:"delivery"`
	Payment           Payment   `json:"payment"`
	Items             []Item    `json:"items"`
	Locale            string    `json:"locale" db:"locale"`
	InternalSignature string    `json:"internal_signature" db:"internal_signature"`
	CustomerId        string    `json:"customer_id" db:"customer_id"`
	DeliveryService   string    `json:"delivery_service" db:"delivery_service"`
	Shardkey          string    `json:"shardkey" db:"shardkey"`
	SmId              int       `json:"sm_id" db:"sm_id"`
	DateCreated       time.Time `json:"date_created" db:"date_created"`
	OofShard          string    `json:"oof_shard" db:"oof_shard"`
}

var (
	cache       = make(map[string]Order)
	databaseURL = "postgres://postgres:12345@localhost:5432/local_db?sslmode=disable"
	db          *sqlx.DB
)

func main() {
	initDB()

	for _, order := range readDB() {
		cache[order.OrderUid] = order
	}
	initNATS()
	defer closeNATS()
	defer closeDB()
	subscription := subscribeToOrders(handleMessage)
	defer func(subscription stan.Subscription) {
		err := subscription.Unsubscribe()
		if err != nil {
			return
		}
	}(subscription)

	// Инициализация HTTP-сервера
	mux := http.NewServeMux()
	mux.HandleFunc("/", HomePage)
	mux.HandleFunc("/record", IdPage)
	mux.HandleFunc("/list/", DataListPage)

	log.Println("server started on port :8080")

	// Запуск HTTP-сервера
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func HomePage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("static/home-page.html")
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal server error", 500)
		return
	}
	err = tmpl.Execute(w, nil)
	if err != nil {
		log.Println(err.Error())
		http.Error(w, "Internal server error", 500)
		return
	}
}

func IdPage(w http.ResponseWriter, r *http.Request) { // страница с записью с заданным id
	needId := r.URL.Query().Get("id")
	if _, ok := cache[needId]; ok {
		b, _ := json.Marshal(cache[needId])
		_, err := w.Write(b)
		if err != nil {
			log.Println(err.Error())
		}
	} else {
		_, err := w.Write([]byte("Запись не найдена"))
		if err != nil {
			log.Println(err.Error())
		}
	}
}

func DataListPage(w http.ResponseWriter, r *http.Request) { // страница вывода списка всех записей
	outputArray := make([]Order, 0)
	for _, elem := range cache {
		outputArray = append(outputArray, elem)
	}

	b, _ := json.Marshal(outputArray)
	_, err := w.Write(b)
	if err != nil {
		log.Println(err.Error())
	}
}

func initDB() {
	var err error

	// Открываем соединение с базой данных
	db, err = sqlx.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal(err)
	}

	// Проверяем соединение
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Connected to the database")
}

func closeDB() {
	// Закрываем соединение с базой данных
	if db != nil {
		db.Close()
	}
}

// Функция для чтения всех данных из базы данных и возврата массива Order
func readDB() []Order {
	var orders []Order

	// Чтение данных из таблицы orders
	rows, err := db.Query(`
        SELECT * FROM orders
    `)
	if err != nil {
		log.Printf("Error executing SQL query: %v\n", err)
		return nil
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Println("Error closing rows:", err)
		}
	}(rows)
	// Цикл по результатам запроса
	for rows.Next() {
		var orderData Order
		err := rows.Scan(
			&orderData.OrderUid,
			&orderData.TrackNumber,
			&orderData.Entry,
			&orderData.Locale,
			&orderData.InternalSignature,
			&orderData.CustomerId,
			&orderData.DeliveryService,
			&orderData.Shardkey,
			&orderData.SmId,
			&orderData.DateCreated,
			&orderData.OofShard,
		)
		if err != nil {
			log.Printf("Error executing SQL query: %v\n", err)
			continue
		}

		// Чтение данных из связанных таблиц
		err = readDelivery(&orderData.Delivery, orderData.CustomerId)
		if err != nil {
			log.Printf("Error executing SQL query: %v\n", err)
			continue
		}

		err = readPayment(&orderData.Payment, orderData.OrderUid)
		if err != nil {
			log.Printf("Error executing SQL query: %v\n", err)
			continue
		}

		err = readItems(&orderData.Items, orderData.OrderUid)
		if err != nil {
			log.Printf("Error executing SQL query: %v\n", err)
			continue
		}

		// Добавление заказа в массив
		orders = append(orders, orderData)
	}

	return orders
}

// Функция для чтения данных из таблицы deliveries
func readDelivery(delivery *Delivery, customerId string) error {
	row := db.QueryRow(`
        SELECT name, phone, zip, city, address, region, email FROM deliveries WHERE customer_id = $1
    `, customerId)

	return row.Scan(
		&delivery.Name,
		&delivery.Phone,
		&delivery.Zip,
		&delivery.City,
		&delivery.Address,
		&delivery.Region,
		&delivery.Email,
	)
}

// Функция для чтения данных из таблицы payments
func readPayment(payment *Payment, orderUID string) error {
	row := db.QueryRow(`
        SELECT transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee FROM payments WHERE order_uid = $1
    `, orderUID)

	return row.Scan(
		&payment.Transaction,
		&payment.RequestId,
		&payment.Currency,
		&payment.Provider,
		&payment.Amount,
		&payment.PaymentDt,
		&payment.Bank,
		&payment.DeliveryCost,
		&payment.GoodsTotal,
		&payment.CustomFee,
	)
}

// Функция для чтения данных из таблицы items
func readItems(items *[]Item, orderUID string) error {
	rows, err := db.Query(`
        SELECT chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand,status FROM items WHERE order_uid = $1
    `, orderUID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var item Item
		err := rows.Scan(
			&item.ChrtId,
			&item.TrackNumber,
			&item.Price,
			&item.Rid,
			&item.Name,
			&item.Sale,
			&item.Size,
			&item.TotalPrice,
			&item.NmId,
			&item.Brand,
			&item.Status,
		)
		if err != nil {
			log.Printf("Error executing SQL query: %v\n", err)
			continue
		}
		*items = append(*items, item)
	}

	return nil
}

func writeToDB(order Order) error {
	// Начало транзакции
	tx, err := db.Begin()
	if err != nil {
		return err
	}

	// Отложенный вызов функции для commit или rollback в зависимости от результата
	defer func() {
		if err != nil {
			// В случае ошибки вызываем откат транзакции
			err := tx.Rollback()
			if err != nil {
				log.Println("Transaction rolled back due to error:", err)
			}
		} else {
			// В случае успешного выполнения транзакции вызываем commit
			err = tx.Commit()
			if err != nil {
				log.Println("Error committing transaction:", err)
			}
		}
	}()

	// Выполнение SQL-запросов в рамках транзакции
	_, err = tx.Exec(`
		INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		order.OrderUid, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature, order.CustomerId, order.DeliveryService, order.Shardkey, order.SmId, order.DateCreated, order.OofShard)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO deliveries (customer_id, name, phone, zip, city, address, region, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, order.CustomerId, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO payments (order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, order.OrderUid, order.Payment.Transaction, order.Payment.RequestId, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee)
	if err != nil {
		return err
	}

	for _, item := range order.Items {
		_, err = tx.Exec(`
		INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, order.OrderUid, item.ChrtId, item.TrackNumber, item.Price, item.Rid, item.Name, item.Sale, item.Size, item.TotalPrice, item.NmId, item.Brand, item.Status)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeCache(order Order) {
	cache[order.OrderUid] = order
}

// Функция обработки сообщений из NATS
func handleMessage(data []byte) {
	// Распаковка данных из NATS-сообщения
	var orderData Order
	err := json.Unmarshal(data, &orderData)
	if err != nil {
		log.Println("Error unmarshalling NATS message:", err)
		return
	}

	// Сохранение данных в базе данных
	err = writeToDB(orderData)
	if err != nil {
		log.Println("Error writing to database:", err)
		return
	}

	// Сохранение данных в кэше
	writeCache(orderData)

	log.Println("Order processed successfully:", orderData.OrderUid)
}
