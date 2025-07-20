CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    age INTEGER,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    product_name TEXT NOT NULL,
    quantity INTEGER DEFAULT 1,
    price DECIMAL(10,2),
    order_date DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id)
);

CREATE TABLE categories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_orders_date ON orders(order_date);

INSERT INTO users (name, email, age) VALUES 
    ('Percy', 'percy@example.com', 28),
    ('James', 'james@example.com', 25),
    ('Remy', 'remy@example.com', 29),
    ('Carney', 'carney@example.com', 31),
    ('Laurel', 'laurel@example.com', 29),
    ('Stefanie', 'stefanie@example.com', 28),
    ('John', 'john@example.com', 45),
    ('Jane', 'jane@example.com', 33),
    ('Bob Miller', 'bob@example.com', 27),
    ('Alice Garcia', 'alice@example.com', 36),
    ('Michael Chen', 'michael@example.com', 40),
    ('Sarah Williams', 'sarah@example.com', 32),
    ('David Taylor', 'david@example.com', 29),
    ('Emma Jackson', 'emma@example.com', 34),
    ('Tom Wilson', 'tom@example.com', 41);

INSERT INTO orders (user_id, product_name, quantity, price) VALUES 
    -- Electronics & Tech
    (1, 'Wireless Bluetooth Headphones', 1, 89.99),
    (2, 'Smart Watch Series 8', 1, 299.99),
    (3, 'Gaming Mechanical Keyboard', 1, 129.99),
    (4, 'USB-C Charging Cable', 3, 19.99),
    (5, 'Portable Power Bank 20000mAh', 1, 45.99),
    (6, 'Wireless Mouse', 1, 34.99),
    (7, '4K Webcam', 1, 159.99),
    (8, 'Bluetooth Speaker', 2, 79.99),
    (9, 'Phone Case', 1, 24.99),
    (10, 'Screen Protector', 2, 12.99),

    -- Office Supplies
    (11, 'Ergonomic Office Chair', 1, 249.99),
    (12, 'Standing Desk Converter', 1, 199.99),
    (13, 'LED Desk Lamp', 1, 49.99),
    (14, 'Notebook Set', 5, 8.99),
    (15, 'Blue Ink Pens', 12, 2.49),
    (1, 'Sticky Notes Pack', 10, 4.99),
    (2, 'Document Folder', 6, 7.99),
    (3, 'Desk Organizer', 1, 29.99),
    (4, 'Whiteboard Markers', 8, 3.99),
    (5, 'Paper Clips Box', 3, 5.99),

    -- Books & Learning
    (6, 'Programming in Python Book', 1, 39.99),
    (7, 'Business Strategy Guide', 1, 29.99),
    (8, 'Online Course Subscription', 1, 99.99),
    (9, 'Technical Manual', 1, 65.99),
    (10, 'Project Management Book', 1, 34.99),
    (11, 'Design Thinking Workbook', 1, 24.99),
    (12, 'Data Analysis Guide', 1, 42.99),
    (13, 'Leadership Handbook', 1, 27.99),
    (14, 'Digital Marketing Book', 1, 31.99),
    (15, 'Time Management Planner', 1, 19.99),

    -- Home & Lifestyle
    (1, 'Coffee Mug', 2, 14.99),
    (2, 'Water Bottle', 1, 22.99),
    (3, 'Desk Plant', 1, 15.99),
    (4, 'Lunch Box', 1, 18.99),
    (5, 'Travel Backpack', 1, 79.99),
    (6, 'Umbrella', 1, 25.99),
    (7, 'Sneakers', 1, 89.99),
    (8, 'T-Shirt', 3, 19.99),
    (9, 'Hoodie', 1, 49.99),
    (10, 'Baseball Cap', 1, 24.99),

    -- Software & Services
    (11, 'Cloud Storage Plan', 12, 9.99),
    (12, 'Password Manager', 12, 3.99),
    (13, 'VPN Service', 12, 8.99),
    (14, 'Antivirus Software', 1, 49.99),
    (15, 'Photo Editing Software', 1, 79.99),
    (1, 'Music Streaming Service', 12, 9.99),
    (2, 'Video Streaming Service', 12, 15.99),
    (3, 'Productivity App Suite', 12, 6.99),
    (4, 'Backup Service', 12, 5.99),
    (5, 'Domain Registration', 1, 12.99),

    -- Food & Beverages
    (6, 'Coffee Beans 1lb', 2, 16.99),
    (7, 'Tea Variety Pack', 1, 24.99),
    (8, 'Energy Bars Box', 1, 29.99),
    (9, 'Protein Powder', 1, 39.99),
    (10, 'Snack Mix', 3, 8.99),
    (11, 'Lunch Delivery', 5, 12.99),
    (12, 'Pizza Order', 1, 18.99),
    (13, 'Sandwich Combo', 2, 9.99),
    (14, 'Salad Bowl', 1, 11.99),
    (15, 'Smoothie', 1, 7.99);

INSERT INTO categories (name, description) VALUES 
    ('Electronics', 'Electronic devices, gadgets, and tech accessories'),
    ('Office Supplies', 'Furniture, stationery, and workplace essentials'),
    ('Books & Learning', 'Educational materials, courses, and reference guides'),
    ('Software & Services', 'Digital subscriptions, apps, and online services'),
    ('Home & Lifestyle', 'Personal items, clothing, and everyday products'),
    ('Food & Beverages', 'Meals, snacks, drinks, and food deliveries'),
    ('Entertainment', 'Games, movies, music, and recreational activities'),
    ('Health & Fitness', 'Wellness products, supplements, and fitness gear'),
    ('Travel & Transport', 'Luggage, travel accessories, and transportation');
