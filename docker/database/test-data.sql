-- 測試資料插入腳本
-- E-commerce Test Data

-- 插入產品分類
INSERT INTO categories (name, description, sort_order) VALUES
('電子產品', '各種電子產品和配件', 1),
('服飾', '男裝、女裝和配件', 2),
('家居用品', '家居裝飾和日常用品', 3),
('書籍', '各類書籍和雜誌', 4),
('運動健身', '運動器材和健身用品', 5);

-- 插入子分類
INSERT INTO categories (name, description, parent_id, sort_order) VALUES
('手機', '智慧手機和平板電腦', (SELECT id FROM categories WHERE name = '電子產品'), 1),
('筆電', '筆記型電腦和配件', (SELECT id FROM categories WHERE name = '電子產品'), 2),
('男裝', '男性服飾', (SELECT id FROM categories WHERE name = '服飾'), 1),
('女裝', '女性服飾', (SELECT id FROM categories WHERE name = '服飾'), 2),
('廚房用品', '烹飪和餐具', (SELECT id FROM categories WHERE name = '家居用品'), 1),
('寢具', '床品和寢具', (SELECT id FROM categories WHERE name = '家居用品'), 2);

-- 插入客戶資料
INSERT INTO customers (email, name, phone, gender, customer_type, total_orders, total_spent, loyalty_points) VALUES
('alice.johnson@email.com', 'Alice Johnson', '+886-912345678', 'female', 'premium', 15, 25000.00, 2500),
('bob.smith@email.com', 'Bob Smith', '+886-987654321', 'male', 'regular', 3, 1200.00, 120),
('carol.davis@email.com', 'Carol Davis', '+886-955511122', 'female', 'vip', 28, 45000.00, 4500),
('david.wilson@email.com', 'David Wilson', '+886-933344455', 'male', 'regular', 1, 350.00, 35),
('emma.brown@email.com', 'Emma Brown', '+886-966677788', 'female', 'premium', 8, 8500.00, 850),
('frank.miller@email.com', 'Frank Miller', '+886-911122233', 'male', 'regular', 5, 2200.00, 220),
('grace.lee@email.com', 'Grace Lee', '+886-944455566', 'female', 'vip', 32, 52000.00, 5200),
('henry.taylor@email.com', 'Henry Taylor', '+886-977788899', 'male', 'regular', 2, 750.00, 75),
('iris.garcia@email.com', 'Iris Garcia', '+886-900011122', 'female', 'premium', 12, 15600.00, 1560),
('jack.anderson@email.com', 'Jack Anderson', '+886-933355577', 'male', 'regular', 4, 1800.00, 180);

-- 更新客戶的註冊日期（模擬不同時間的註冊）
UPDATE customers SET registration_date = CURRENT_TIMESTAMP - INTERVAL '1 year' WHERE id <= 3;
UPDATE customers SET registration_date = CURRENT_TIMESTAMP - INTERVAL '6 months' WHERE id > 3 AND id <= 6;
UPDATE customers SET registration_date = CURRENT_TIMESTAMP - INTERVAL '2 months' WHERE id > 6;

-- 插入客戶地址
INSERT INTO customer_addresses (customer_id, address_type, is_default, street_address, city, postal_code, country) VALUES
(1, 'shipping', true, '台北市中山區中山北路一段 123 號', '台北市', '104', 'Taiwan'),
(1, 'billing', true, '台北市中山區中山北路一段 123 號', '台北市', '104', 'Taiwan'),
(2, 'shipping', true, '新北市板橋區文化路一段 456 號', '新北市', '220', 'Taiwan'),
(3, 'shipping', true, '台中市西區台灣大道二段 789 號', '台中市', '403', 'Taiwan'),
(4, 'shipping', true, '高雄市苓雅區四維路 321 號', '高雄市', '802', 'Taiwan'),
(5, 'shipping', true, '台南市東區大學路 654 號', '台南市', '701', 'Taiwan');

-- 插入產品資料
INSERT INTO products (sku, name, description, category_id, price, cost_price, inventory_quantity, tags, is_featured) VALUES
('PHONE-IP15-128', 'iPhone 15 Pro 128GB', '最新款 iPhone 15 Pro，搭載 A17 Pro 晶片，128GB 儲存空間', (SELECT id FROM categories WHERE name = '手機'), 35900.00, 28000.00, 50, ARRAY['手機', 'Apple', '5G'], true),
('PHONE-S23-256', 'Samsung Galaxy S23 Ultra 256GB', '三星 Galaxy S23 Ultra，S Pen 支援，256GB 儲存空間', (SELECT id FROM categories WHERE name = '手機'), 32900.00, 26000.00, 30, ARRAY['手機', 'Samsung', '5G', 'S Pen'], true),
('LAPTOP-MBP14', 'MacBook Pro 14吋 M2', 'MacBook Pro 14吋，搭載 M2 晶片，16GB RAM，512GB SSD', (SELECT id FROM categories WHERE name = '筆電'), 89900.00, 75000.00, 20, ARRAY['筆電', 'Apple', 'M2'], true),
('TSHIRT-COTTON-M', '純棉圓領T恤 男款 M碼', '100% 純棉材質，舒適透氣，經典圓領設計', (SELECT id FROM categories WHERE name = '男裝'), 450.00, 180.00, 200, ARRAY['T恤', '男裝', '純棉'], false),
('TSHIRT-COTTON-W', '純棉圓領T恤 女款 M碼', '100% 純棉材質，修身版型，時尚設計', (SELECT id FROM categories WHERE name = '女裝'), 480.00, 200.00, 150, ARRAY['T恤', '女裝', '純棉'], false),
('COFFEE-MAKER', '精品咖啡機', '全自動咖啡機，支援磨豆功能，一鍵製作美味咖啡', (SELECT id FROM categories WHERE name = '廚房用品'), 8900.00, 5200.00, 25, ARRAY['咖啡機', '廚房', '精品'], false),
('DUVET-QUEEN', '羽絨被 皇后尺寸', '90% 白鵝絨填充，保暖輕盈，皇后尺寸', (SELECT id FROM categories WHERE name = '寢具'), 12800.00, 6800.00, 15, ARRAY['羽絨被', '寢具', '保暖'], false),
('BOOK-PROG-GO', 'Go 程式設計實戰', 'Go 語言程式設計入門到進階實戰指南', (SELECT id FROM categories WHERE name = '書籍'), 680.00, 250.00, 100, ARRAY['程式設計', 'Go', '技術書'], false),
('YOGA-MAT', '專業瑜伽墊', '環保 TPE 材質，防滑設計，6mm 厚度', (SELECT id FROM categories WHERE name = '運動健身'), 1200.00, 450.00, 80, ARRAY['瑜伽墊', '健身', '瑜伽'], false),
('HEADPHONES-WF', '無線耳機', '主動降噪技術，40 小時續航，快速充電', (SELECT id FROM categories WHERE name = '手機'), 5800.00, 3200.00, 60, ARRAY['耳機', '無線', '降噪'], true);

-- 插入產品變體（以 iPhone 15 為例）
INSERT INTO product_variants (product_id, sku, name, price, cost_price, option1, option2, inventory_quantity) VALUES
(1, 'PHONE-IP15-128-BLK', 'iPhone 15 Pro 128GB 黑色', 35900.00, 28000.00, '黑色', '128GB', 20),
(1, 'PHONE-IP15-128-BLU', 'iPhone 15 Pro 128GB 藍色', 35900.00, 28000.00, '藍色', '128GB', 15),
(1, 'PHONE-IP15-128-WHT', 'iPhone 15 Pro 128GB 白色', 35900.00, 28000.00, '白色', '128GB', 15);

-- 插入訂單資料
INSERT INTO orders (order_number, customer_id, email, status, payment_status, fulfillment_status, subtotal, tax_amount, shipping_amount, total, currency, payment_method, shipping_method, ordered_at) VALUES
('ORD-2024-001', 1, 'alice.johnson@email.com', 'delivered', 'paid', 'fulfilled', 36500.00, 2920.00, 150.00, 39570.00, 'TWD', 'credit_card', 'standard', CURRENT_TIMESTAMP - INTERVAL '30 days'),
('ORD-2024-002', 2, 'bob.smith@email.com', 'shipped', 'paid', 'fulfilled', 1200.00, 96.00, 80.00, 1376.00, 'TWD', 'credit_card', 'standard', CURRENT_TIMESTAMP - INTERVAL '15 days'),
('ORD-2024-003', 3, 'carol.davis@email.com', 'processing', 'paid', 'partial', 33500.00, 2680.00, 0.00, 36180.00, 'TWD', 'bank_transfer', 'express', CURRENT_TIMESTAMP - INTERVAL '3 days'),
('ORD-2024-004', 4, 'david.wilson@email.com', 'confirmed', 'paid', 'unfulfilled', 350.00, 28.00, 80.00, 458.00, 'TWD', 'credit_card', 'standard', CURRENT_TIMESTAMP - INTERVAL '1 day'),
('ORD-2024-005', 5, 'emma.brown@email.com', 'pending', 'pending', 'unfulfilled', 8900.00, 712.00, 150.00, 9762.00, 'TWD', 'credit_card', 'express', CURRENT_TIMESTAMP - INTERVAL '2 hours');

-- 插入訂單項目
INSERT INTO order_items (order_id, product_id, variant_id, sku, name, quantity, price, total) VALUES
(1, 1, 1, 'PHONE-IP15-128-BLK', 'iPhone 15 Pro 128GB 黑色', 1, 35900.00, 35900.00),
(1, 10, NULL, 'HEADPHONES-WF', '無線耳機', 1, 5800.00, 5800.00),
(2, 4, NULL, 'TSHIRT-COTTON-M', '純棉圓領T恤 男款 M碼', 2, 450.00, 900.00),
(2, 9, NULL, 'YOGA-MAT', '專業瑜伽墊', 1, 1200.00, 1200.00),
(3, 2, NULL, 'PHONE-S23-256', 'Samsung Galaxy S23 Ultra 256GB', 1, 32900.00, 32900.00),
(3, 6, NULL, 'COFFEE-MAKER', '精品咖啡機', 1, 8900.00, 8900.00),
(4, 4, NULL, 'TSHIRT-COTTON-M', '純棉圓領T恤 男款 M碼', 1, 450.00, 450.00),
(5, 6, NULL, 'COFFEE-MAKER', '精品咖啡機', 1, 8900.00, 8900.00);

-- 插入付款記錄
INSERT INTO payments (order_id, amount, payment_method, transaction_id, status, processed_at) VALUES
(1, 39570.00, 'credit_card', 'txn_abc123', 'completed', CURRENT_TIMESTAMP - INTERVAL '30 days'),
(2, 1376.00, 'credit_card', 'txn_def456', 'completed', CURRENT_TIMESTAMP - INTERVAL '15 days'),
(3, 36180.00, 'bank_transfer', 'txn_ghi789', 'completed', CURRENT_TIMESTAMP - INTERVAL '3 days'),
(4, 458.00, 'credit_card', 'txn_jkl012', 'completed', CURRENT_TIMESTAMP - INTERVAL '1 day');

-- 插入物流記錄
INSERT INTO shipments (order_id, tracking_number, carrier, status, shipped_at, estimated_delivery) VALUES
(1, 'SF123456789', 'SF Express', 'delivered', CURRENT_TIMESTAMP - INTERVAL '25 days', CURRENT_TIMESTAMP - INTERVAL '20 days'),
(2, 'SF987654321', 'SF Express', 'shipped', CURRENT_TIMESTAMP - INTERVAL '10 days', CURRENT_TIMESTAMP + INTERVAL '2 days'),
(3, 'DHL555666777', 'DHL', 'in_transit', CURRENT_TIMESTAMP - INTERVAL '2 days', CURRENT_TIMESTAMP + INTERVAL '3 days');

-- 插入評價資料
INSERT INTO reviews (product_id, customer_id, order_id, rating, title, content, is_verified, status) VALUES
(1, 1, 1, 5, '非常滿意！', 'iPhone 15 Pro 表現超乎預期，相機和效能都非常棒！', true, 'approved'),
(4, 2, 2, 4, '品質不錯', '純棉材質穿起來很舒服，尺碼也很合身', true, 'approved'),
(9, 2, 2, 5, '瑜伽墊很棒', '防滑效果很好，厚度適中，非常適合居家瑜伽', true, 'approved'),
(2, 3, 3, 5, '三星手機真心不錯', 'S Pen 功能很實用，拍照效果也很好', true, 'approved'),
(6, 3, 3, 4, '咖啡機好用', '磨豆功能很方便，咖啡味道也很棒', true, 'approved'),
(4, 4, 4, 3, '一般般', '衣服還可以，但有點薄', true, 'approved');

-- 插入優惠券資料
INSERT INTO coupons (code, name, description, discount_type, discount_value, minimum_amount, usage_limit, starts_at, expires_at) VALUES
('WELCOME10', '新用戶優惠', '新註冊用戶首次購物9折', 'percentage', 10.00, 500.00, 1000, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year'),
('SUMMER20', '夏日特惠', '全館8折優惠', 'percentage', 20.00, 1000.00, 500, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '3 months'),
('FLASH500', '閃購優惠', '滿3000折500', 'fixed_amount', 500.00, 3000.00, 200, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 week');

-- 插入庫存異動記錄
INSERT INTO inventory_transactions (product_id, transaction_type, quantity, previous_quantity, new_quantity, reference_type, reference_id, notes) VALUES
(1, 'sale', -1, 50, 49, 'order', 1, 'iPhone 15 Pro 銷售'),
(10, 'sale', -1, 60, 59, 'order', 1, '無線耳機銷售'),
(4, 'sale', -2, 200, 198, 'order', 2, 'T恤銷售'),
(9, 'sale', -1, 80, 79, 'order', 2, '瑜伽墊銷售'),
(2, 'sale', -1, 30, 29, 'order', 3, 'Galaxy S23 銷售'),
(6, 'sale', -1, 25, 24, 'order', 3, '咖啡機銷售'),
(4, 'sale', -1, 198, 197, 'order', 4, 'T恤銷售'),
(6, 'adjustment', 5, 24, 29, 'adjustment', NULL, '補貨調整');

-- 更新產品庫存數量以反映銷售
UPDATE products SET inventory_quantity = 49 WHERE id = 1;
UPDATE products SET inventory_quantity = 59 WHERE id = 10;
UPDATE products SET inventory_quantity = 197 WHERE id = 4;
UPDATE products SET inventory_quantity = 79 WHERE id = 9;
UPDATE products SET inventory_quantity = 29 WHERE id = 2;
UPDATE products SET inventory_quantity = 29 WHERE id = 6;

-- 更新客戶的訂單統計
UPDATE customers SET total_orders = 1, total_spent = 39570.00 WHERE id = 1;
UPDATE customers SET total_orders = 1, total_spent = 1376.00 WHERE id = 2;
UPDATE customers SET total_orders = 1, total_spent = 36180.00 WHERE id = 3;
UPDATE customers SET total_orders = 1, total_spent = 458.00 WHERE id = 4;