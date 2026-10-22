-- Deterministic benchmark data: 1,000 fruits, 100 stores, 10,000 prices.
-- Production starts do not load the tiny test fixtures in each Java project.
CREATE SEQUENCE fruits_seq START WITH 1001 INCREMENT BY 1;
CREATE SEQUENCE stores_seq START WITH 101 INCREMENT BY 1;

CREATE TABLE fruits (
    id bigint PRIMARY KEY DEFAULT nextval('fruits_seq'),
    description varchar(255),
    name varchar(255) NOT NULL UNIQUE
);
ALTER SEQUENCE fruits_seq OWNED BY fruits.id;

CREATE TABLE stores (
    id bigint PRIMARY KEY DEFAULT nextval('stores_seq'),
    address varchar(255) NOT NULL,
    city varchar(255) NOT NULL,
    country varchar(255) NOT NULL,
    currency varchar(255) NOT NULL,
    name varchar(255) NOT NULL UNIQUE
);
ALTER SEQUENCE stores_seq OWNED BY stores.id;

CREATE TABLE store_fruit_prices (
    fruit_id bigint NOT NULL REFERENCES fruits(id),
    store_id bigint NOT NULL REFERENCES stores(id),
    price numeric(12,2) NOT NULL,
    PRIMARY KEY (fruit_id, store_id)
);
CREATE INDEX store_fruit_prices_store_idx ON store_fruit_prices(store_id);

INSERT INTO fruits (id, name, description)
SELECT n,
    CASE WHEN n <= 10 THEN (ARRAY['Apple','Pear','Banana','Orange','Strawberry',
        'Mango','Grape','Pineapple','Watermelon','Kiwi'])[n]
        ELSE 'Fruit' || lpad(n::text, 4, '0') END,
    'Seasonal produce ' || n || ': fresh harvest, quality checked, available from regional stores.'
FROM generate_series(1, 1000) AS n;

INSERT INTO stores (id, name, address, city, country, currency)
SELECT n, 'Store ' || n, (n * 17) || ' Market Street',
    (ARRAY['Zurich','Berlin','Paris','London','New York'])[(n - 1) % 5 + 1],
    (ARRAY['Switzerland','Germany','France','UK','USA'])[(n - 1) % 5 + 1],
    (ARRAY['CHF','EUR','EUR','GBP','USD'])[(n - 1) % 5 + 1]
FROM generate_series(1, 100) AS n;

INSERT INTO store_fruit_prices (fruit_id, store_id, price)
SELECT fruit_id, ((fruit_id * 7 + slot * 11) % 100) + 1,
    (50 + (fruit_id * 31 + slot * 19) % 1500)::numeric / 100
FROM generate_series(1, 1000) AS fruit_id CROSS JOIN generate_series(0, 9) AS slot;

ANALYZE;
