CREATE TABLE orders
(
    order_uid          varchar PRIMARY KEY NOT NULL,
    track_number       varchar,
    entry              varchar,
    locale             varchar,
    internal_signature varchar,
    customer_id        varchar UNIQUE NOT NULL,
    delivery_service   varchar,
    shardkey           varchar,
    sm_id              smallint,
    date_created       timestamp,
    oof_shard          varchar
);

CREATE TABLE deliveries
(
    customer_id varchar PRIMARY KEY NOT NULL REFERENCES orders(customer_id) ON DELETE CASCADE,
    name    varchar,
    phone   varchar,
    zip     varchar,
    city    varchar,
    address varchar,
    region  varchar,
    email   varchar
);

CREATE TABLE payments
(
    order_uid     varchar PRIMARY KEY NOT NULL REFERENCES orders(order_uid) ON DELETE CASCADE,
    transaction   varchar,
    request_id    varchar,
    currency      varchar,
    provider      varchar,
    amount        int,
    payment_dt    int,
    bank          varchar,
    delivery_cost int,
    goods_total   int,
    custom_fee    int
);

CREATE TABLE items
(
    item_id	     serial PRIMARY KEY NOT NULL,
    order_uid    varchar NOT NULL REFERENCES orders(order_uid) ON DELETE CASCADE,
    chrt_id      int,
    track_number varchar,
    price        int,
    rid          varchar,
    name         varchar,
    sale         int,
    size         varchar,
    total_price  int,
    nm_id        int,
    brand        varchar,
    status       int
);


INSERT INTO orders (order_uid, track_number, entry, locale, internal_signature, customer_id, delivery_service, shardkey, sm_id, date_created, oof_shard)
VALUES ('b563feb7b2b84b6test', 'WBILMTESTTRACK', 'WBIL', 'en', '', 'test', 'meest', '9', 99, '2021-11-26T06:22:19Z', '1');
INSERT INTO deliveries (customer_id, name, phone, zip, city, address, region, email)
VALUES ('test', 'Test Testov', '+9720000000', '2639809', 'Kiryat Mozkin', 'Ploshad Mira 15', 'Kraiot', 'test@gmail.com');
INSERT INTO payments (order_uid, transaction, request_id, currency, provider, amount, payment_dt, bank, delivery_cost, goods_total, custom_fee)
VALUES ('b563feb7b2b84b6test', 'b563feb7b2b84b6test', '', 'USD', 'wbpay', 1817, 1637907727, 'alpha', 1500, 317, 0);
INSERT INTO items (order_uid, chrt_id, track_number, price, rid, name, sale, size, total_price, nm_id, brand, status)
VALUES ('b563feb7b2b84b6test', 9934930, 'WBILMTESTTRACK', 453, 'ab4219087a764ae0btest', 'Mascaras', 30, '0', 317, 2389212, 'Vivienne Sabo', 202);
