CREATE TABLE catalog_items (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    age_range_code TEXT NOT NULL,
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    marketplace_search_url TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX catalog_items_filter_idx ON catalog_items (age_range_code, category);

INSERT INTO catalog_items (age_range_code, category, title, marketplace_search_url) VALUES
    ('0m', 'clothes', 'Боди с длинным рукавом для новорождённого', 'https://www.ozon.ru/search/?text=боди+для+новорожденного'),
    ('3m', 'clothes', 'Слипы на кнопках, набор из трёх', 'https://www.ozon.ru/search/?text=слипы+для+малыша'),
    ('6m', 'toys', 'Мягкая погремушка-прорезыватель', 'https://www.ozon.ru/search/?text=погремушка+прорезыватель'),
    ('9m', 'toys', 'Пирамидка с крупными кольцами', 'https://www.ozon.ru/search/?text=пирамидка+для+малышей'),
    ('12m', 'books', 'Картонная книжка-непромокашка для ванной', 'https://www.ozon.ru/search/?text=книжка+непромокашка'),
    ('18m', 'toys', 'Сортер с крупными деталями', 'https://www.ozon.ru/search/?text=сортер+для+малышей'),
    ('18m', 'clothes', 'Резиновые сапожки на флисовой подкладке', 'https://www.ozon.ru/search/?text=резиновые+сапожки+детские'),
    ('24m', 'sport', 'Беговел для самых маленьких', 'https://www.ozon.ru/search/?text=беговел+детский'),
    ('24m', 'books', 'Книжка с окошками про животных', 'https://www.ozon.ru/search/?text=книжка+с+окошками'),
    ('30m', 'toys', 'Конструктор с крупными блоками', 'https://www.ozon.ru/search/?text=конструктор+крупные+блоки'),
    ('3y', 'clothes', 'Комбинезон для прогулок в межсезонье', 'https://www.ozon.ru/search/?text=комбинезон+демисезонный+детский'),
    ('4y', 'books', 'Сборник сказок с крупными иллюстрациями', 'https://www.ozon.ru/search/?text=сборник+сказок+для+детей'),
    ('5y', 'sport', 'Самокат с ручным тормозом', 'https://www.ozon.ru/search/?text=самокат+детский'),
    ('6y', 'toys', 'Настольная игра на внимание', 'https://www.ozon.ru/search/?text=настольная+игра+для+детей'),
    ('7y', 'books', 'Книга-квест для самостоятельного чтения', 'https://www.ozon.ru/search/?text=книга+квест+для+детей'),
    ('9y', 'sport', 'Скейтборд для начинающих', 'https://www.ozon.ru/search/?text=скейтборд+для+начинающих'),
    ('11y', 'clothes', 'Куртка-дождевик подростковая', 'https://www.ozon.ru/search/?text=куртка+дождевик+подростковая'),
    ('12y+', 'sport', 'Ролики для активного отдыха', 'https://www.ozon.ru/search/?text=ролики+подростковые');
