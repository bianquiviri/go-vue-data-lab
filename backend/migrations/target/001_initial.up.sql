-- Initial schema for lab_target
CREATE TABLE IF NOT EXISTS customers (
  id UUID PRIMARY KEY,
  first_name VARCHAR(100) NOT NULL,
  last_name VARCHAR(100) NOT NULL,
  email VARCHAR(255) NOT NULL UNIQUE,
  country_code CHAR(2) NOT NULL,
  city VARCHAR(100) NOT NULL,
  state VARCHAR(100),
  created_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS products (
  id UUID PRIMARY KEY,
  sku VARCHAR(50) NOT NULL UNIQUE,
  name VARCHAR(255) NOT NULL,
  category VARCHAR(100) NOT NULL,
  price DECIMAL(10, 2) NOT NULL CHECK (price >= 0),
  stock INTEGER NOT NULL CHECK (stock >= 0),
  status VARCHAR(20) NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS orders (
  id UUID PRIMARY KEY,
  order_number VARCHAR(50) NOT NULL UNIQUE,
  customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
  status VARCHAR(30) NOT NULL,
  total_amount DECIMAL(10, 2) NOT NULL,
  currency CHAR(3) NOT NULL,
  shipping_address JSONB,
  order_date TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS order_items (
  id BIGINT PRIMARY KEY,
  order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  product_id UUID NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
  quantity INTEGER NOT NULL CHECK (quantity > 0),
  unit_price DECIMAL(10, 2) NOT NULL CHECK (unit_price >= 0),
  subtotal DECIMAL(10, 2) NOT NULL CHECK (subtotal >= 0)
);

CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
CREATE INDEX IF NOT EXISTS idx_products_sku ON products(sku);
CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
