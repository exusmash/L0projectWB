package main

import (
	"database/sql"
	"encoding/json"
	_ "github.com/lib/pq"
	"github.com/nats-io/stan.go"
	"log"
	"net/http"
	"strings"
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
	databaseURL = "postgres://postgres:12345@localhost:5432/mypostgres?sslmode=disable"
	cache       = make(map[string]Order)
	db          *sql.DB
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
	http.HandleFunc("/order/", getOrderHandler)

	// Запуск HTTP-сервера
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func getOrderHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем id из URL
	id := strings.TrimPrefix(r.URL.Path, "/order/")

	// Проверяем, есть ли заказ в кэше
	order, ok := cache[id]
	if !ok {
		// Если заказа нет в кэше, пытаемся прочитать его из базы данных
		order, err := readOrderById(id)
		if err != nil {
			// Если не удалось прочитать из БД, возвращаем ошибку
			http.Error(w, "Order not found", http.StatusNotFound)
			return
		}

		// Сохраняем заказ в кэше
		cache[id] = order
	}

	// Преобразуем заказ в JSON и отправляем клиенту
	jsonData, err := json.Marshal(order)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func readOrderById(id string) (Order, error) {
	var orderData Order
	err := db.QueryRow("SELECT * FROM orders WHERE order_uid = $1", id).Scan(
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
		&orderData.OofShard)
	return orderData, err
}

func initDB() {
	var err error

	// Открываем соединение с базой данных
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal(err)
	}

	// Проверяем соединение
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
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

	// Начало транзакции чтения
	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return nil
	}
	defer tx.Rollback() // Откат транзакции в случае ошибки

	// Чтение данных из таблицы orders
	rows, err := tx.Query(`
		SELECT * FROM orders
	`)
	if err != nil {
		log.Println(err)
		return nil
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

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
			log.Println(err)
			continue
		}

		// Чтение данных из связанных таблиц
		err = readDelivery(tx, &orderData.Delivery, orderData.OrderUid)
		if err != nil {
			log.Println(err)
			continue
		}

		err = readPayment(tx, &orderData.Payment, orderData.OrderUid)
		if err != nil {
			log.Println(err)
			continue
		}

		err = readItems(tx, &orderData.Items, orderData.OrderUid)
		if err != nil {
			log.Println(err)
			continue
		}

		// Добавление заказа в массив
		orders = append(orders, orderData)
	}

	return orders
}

// Функция для чтения данных из таблицы deliveries
func readDelivery(tx *sql.Tx, delivery *Delivery, orderUID string) error {
	row := tx.QueryRow(`
		SELECT * FROM deliveries WHERE customer_id = $1
	`, orderUID)

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
func readPayment(tx *sql.Tx, payment *Payment, orderUID string) error {
	row := tx.QueryRow(`
		SELECT * FROM payments WHERE order_uid = $1
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
func readItems(tx *sql.Tx, items *[]Item, orderUID string) error {
	rows, err := tx.Query(`
		SELECT * FROM items WHERE order_uid = $1
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
			log.Println(err)
			continue
		}
		*items = append(*items, item)
	}

	return nil
}

// Функция для записи данных заказа в базу данных
func writeToDB(order Order) error {
	// Выполнение SQL-запроса для вставки или обновления данных в БД
	_, err := db.Exec(`INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) ON CONFLICT (order_uid) DO NOTHING`, order.OrderUid, order.TrackNumber, order.Entry, order.Locale, order.InternalSignature, order.CustomerId, order.DeliveryService, order.Shardkey, order.SmId, order.DateCreated, order.OofShard)
	if err != nil {
		return err
	}

	_, err = db.Exec(`INSERT INTO deliveries (customer_id, name, phone, zip, city, address, region, email)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (customer_id) DO NOTHING
	`, order.OrderUid, order.Delivery.Name, order.Delivery.Phone, order.Delivery.Zip, order.Delivery.City, order.Delivery.Address, order.Delivery.Region, order.Delivery.Email)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		INSERT INTO payments (order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (order_uid) DO NOTHING
	`, order.OrderUid, order.Payment.Transaction, order.Payment.RequestId, order.Payment.Currency, order.Payment.Provider, order.Payment.Amount, order.Payment.PaymentDt, order.Payment.Bank, order.Payment.DeliveryCost, order.Payment.GoodsTotal, order.Payment.CustomFee)
	if err != nil {
		return err
	}

	for _, item := range order.Items {
		_, err = db.Exec(`
		INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (order_uid, track_number) DO NOTHING
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

}
