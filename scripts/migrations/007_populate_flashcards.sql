-- +goose Up
-- +goose StatementBegin

-- Заполнение таблицы flashcards карточками для каждого уровня
-- 50 карточек для каждого уровня (beginner, intermediate, advanced)

-- BEGINNER LEVEL CARDS (50 карточек)
INSERT INTO flashcards (word, translation, example, level, category) VALUES
-- Общие слова
('hello', 'привет', 'Hello! How are you?', 'beginner', 'general'),
('goodbye', 'до свидания', 'Goodbye! See you later.', 'beginner', 'general'),
('yes', 'да', 'Yes, I understand.', 'beginner', 'general'),
('no', 'нет', 'No, I don''t know.', 'beginner', 'general'),
('please', 'пожалуйста', 'Please help me.', 'beginner', 'general'),
('thank you', 'спасибо', 'Thank you very much.', 'beginner', 'general'),
('sorry', 'извините', 'Sorry, I''m late.', 'beginner', 'general'),
('excuse me', 'извините', 'Excuse me, where is the bank?', 'beginner', 'general'),
('good morning', 'доброе утро', 'Good morning! How are you?', 'beginner', 'general'),
('good evening', 'добрый вечер', 'Good evening! Nice to meet you.', 'beginner', 'general'),

-- Семья
('family', 'семья', 'I love my family.', 'beginner', 'general'),
('mother', 'мама', 'My mother is kind.', 'beginner', 'general'),
('father', 'папа', 'My father works hard.', 'beginner', 'general'),
('sister', 'сестра', 'I have a younger sister.', 'beginner', 'general'),
('brother', 'брат', 'My brother is tall.', 'beginner', 'general'),
('son', 'сын', 'He is my son.', 'beginner', 'general'),
('daughter', 'дочь', 'She is my daughter.', 'beginner', 'general'),
('baby', 'ребенок', 'The baby is sleeping.', 'beginner', 'general'),
('grandmother', 'бабушка', 'My grandmother bakes cookies.', 'beginner', 'general'),
('grandfather', 'дедушка', 'My grandfather tells stories.', 'beginner', 'general'),

-- Цвета
('red', 'красный', 'The apple is red.', 'beginner', 'general'),
('blue', 'синий', 'The sky is blue.', 'beginner', 'general'),
('green', 'зеленый', 'The grass is green.', 'beginner', 'general'),
('yellow', 'желтый', 'The sun is yellow.', 'beginner', 'general'),
('black', 'черный', 'The night is black.', 'beginner', 'general'),
('white', 'белый', 'The snow is white.', 'beginner', 'general'),
('orange', 'оранжевый', 'The carrot is orange.', 'beginner', 'general'),
('purple', 'фиолетовый', 'The grape is purple.', 'beginner', 'general'),
('pink', 'розовый', 'The flower is pink.', 'beginner', 'general'),
('brown', 'коричневый', 'The chocolate is brown.', 'beginner', 'general'),

-- Числа
('one', 'один', 'I have one cat.', 'beginner', 'general'),
('two', 'два', 'I have two dogs.', 'beginner', 'general'),
('three', 'три', 'I have three books.', 'beginner', 'general'),
('four', 'четыре', 'There are four seasons.', 'beginner', 'general'),
('five', 'пять', 'I have five fingers.', 'beginner', 'general'),
('six', 'шесть', 'The clock shows six.', 'beginner', 'general'),
('seven', 'семь', 'There are seven days in a week.', 'beginner', 'general'),
('eight', 'восемь', 'I have eight friends.', 'beginner', 'general'),
('nine', 'девять', 'The cat has nine lives.', 'beginner', 'general'),
('ten', 'десять', 'I can count to ten.', 'beginner', 'general'),

-- Еда
('bread', 'хлеб', 'I eat bread every day.', 'beginner', 'food'),
('milk', 'молоко', 'I drink milk for breakfast.', 'beginner', 'food'),
('water', 'вода', 'Water is essential for life.', 'beginner', 'food'),
('apple', 'яблоко', 'An apple a day keeps the doctor away.', 'beginner', 'food'),
('banana', 'банан', 'I like yellow bananas.', 'beginner', 'food'),
('meat', 'мясо', 'I eat meat for dinner.', 'beginner', 'food'),
('fish', 'рыба', 'Fish is healthy food.', 'beginner', 'food'),
('rice', 'рис', 'Rice is popular in Asia.', 'beginner', 'food'),
('egg', 'яйцо', 'I eat eggs for breakfast.', 'beginner', 'food'),
('cheese', 'сыр', 'I like cheese on pizza.', 'beginner', 'food')
ON CONFLICT (word, level, category) DO NOTHING;

-- INTERMEDIATE LEVEL CARDS (50 карточек)
INSERT INTO flashcards (word, translation, example, level, category) VALUES
-- Бизнес слова
('meeting', 'встреча', 'We have a meeting at 3 PM.', 'intermediate', 'business'),
('presentation', 'презентация', 'She gave an excellent presentation.', 'intermediate', 'business'),
('deadline', 'срок', 'The deadline is next Friday.', 'intermediate', 'business'),
('budget', 'бюджет', 'We need to stay within budget.', 'intermediate', 'business'),
('strategy', 'стратегия', 'Our strategy is to expand globally.', 'intermediate', 'business'),
('client', 'клиент', 'The client is very satisfied.', 'intermediate', 'business'),
('project', 'проект', 'This project will take three months.', 'intermediate', 'business'),
('team', 'команда', 'Our team works well together.', 'intermediate', 'business'),
('goal', 'цель', 'Our goal is to increase sales.', 'intermediate', 'business'),
('success', 'успех', 'Success comes from hard work.', 'intermediate', 'business'),

-- Путешествия
('passport', 'паспорт', 'Don''t forget your passport.', 'intermediate', 'travel'),
('luggage', 'багаж', 'My luggage is very heavy.', 'intermediate', 'travel'),
('reservation', 'бронирование', 'I made a hotel reservation.', 'intermediate', 'travel'),
('destination', 'направление', 'Paris is my favorite destination.', 'intermediate', 'travel'),
('journey', 'путешествие', 'The journey was long but interesting.', 'intermediate', 'travel'),
('adventure', 'приключение', 'We had an amazing adventure.', 'intermediate', 'travel'),
('explore', 'исследовать', 'I love to explore new places.', 'intermediate', 'travel'),
('experience', 'опыт', 'Travel gives you new experiences.', 'intermediate', 'travel'),
('culture', 'культура', 'I love learning about new cultures.', 'intermediate', 'travel'),
('tradition', 'традиция', 'Every country has its traditions.', 'intermediate', 'travel'),

-- Технологии
('software', 'программное обеспечение', 'This software is very useful.', 'intermediate', 'technology'),
('hardware', 'аппаратное обеспечение', 'The hardware needs to be upgraded.', 'intermediate', 'technology'),
('database', 'база данных', 'We store information in a database.', 'intermediate', 'technology'),
('algorithm', 'алгоритм', 'This algorithm is very efficient.', 'intermediate', 'technology'),
('interface', 'интерфейс', 'The user interface is intuitive.', 'intermediate', 'technology'),
('network', 'сеть', 'The network is down.', 'intermediate', 'technology'),
('server', 'сервер', 'The server crashed yesterday.', 'intermediate', 'technology'),
('application', 'приложение', 'This application helps me work.', 'intermediate', 'technology'),
('platform', 'платформа', 'We use a cloud platform.', 'intermediate', 'technology'),
('framework', 'фреймворк', 'This framework is popular.', 'intermediate', 'technology'),

-- Образование
('knowledge', 'знание', 'Knowledge is power.', 'intermediate', 'education'),
('education', 'образование', 'Education is important for success.', 'intermediate', 'education'),
('learning', 'обучение', 'Learning never stops.', 'intermediate', 'education'),
('research', 'исследование', 'Research shows that exercise is good.', 'intermediate', 'education'),
('analysis', 'анализ', 'The analysis revealed interesting results.', 'intermediate', 'education'),
('theory', 'теория', 'This theory explains the phenomenon.', 'intermediate', 'education'),
('method', 'метод', 'This method is very effective.', 'intermediate', 'education'),
('approach', 'подход', 'We need a different approach.', 'intermediate', 'education'),
('solution', 'решение', 'We found a good solution.', 'intermediate', 'education'),
('challenge', 'вызов', 'This is an interesting challenge.', 'intermediate', 'education'),

-- Здоровье
('nutrition', 'питание', 'Good nutrition is important for health.', 'intermediate', 'health'),
('exercise', 'упражнение', 'Regular exercise keeps you fit.', 'intermediate', 'health'),
('wellness', 'благополучие', 'Wellness includes physical and mental health.', 'intermediate', 'health'),
('prevention', 'профилактика', 'Prevention is better than cure.', 'intermediate', 'health'),
('treatment', 'лечение', 'The treatment was successful.', 'intermediate', 'health'),
('recovery', 'восстановление', 'Recovery takes time.', 'intermediate', 'health'),
('symptom', 'симптом', 'What are your symptoms?', 'intermediate', 'health'),
('diagnosis', 'диагноз', 'The doctor made a diagnosis.', 'intermediate', 'health'),
('therapy', 'терапия', 'Therapy helped me a lot.', 'intermediate', 'health'),
('medicine', 'лекарство', 'Take your medicine regularly.', 'intermediate', 'health')
ON CONFLICT (word, level, category) DO NOTHING;

-- ADVANCED LEVEL CARDS (50 карточек)
INSERT INTO flashcards (word, translation, example, level, category) VALUES
-- Сложные бизнес термины
('entrepreneurship', 'предпринимательство', 'Entrepreneurship requires innovation and risk-taking.', 'advanced', 'business'),
('sustainability', 'устойчивость', 'Sustainability is crucial for long-term business success.', 'advanced', 'business'),
('optimization', 'оптимизация', 'Process optimization can reduce costs significantly.', 'advanced', 'business'),
('innovation', 'инновация', 'Innovation drives market growth.', 'advanced', 'business'),
('disruption', 'нарушение', 'Digital disruption is changing traditional industries.', 'advanced', 'business'),
('scalability', 'масштабируемость', 'The business model needs to be scalable.', 'advanced', 'business'),
('synergy', 'синергия', 'The merger created great synergy between companies.', 'advanced', 'business'),
('leverage', 'рычаг', 'We can leverage our existing customer base.', 'advanced', 'business'),
('paradigm', 'парадигма', 'The industry is experiencing a paradigm shift.', 'advanced', 'business'),
('arbitrage', 'арбитраж', 'Arbitrage opportunities exist in volatile markets.', 'advanced', 'business'),

-- Продвинутые технологии
('artificial intelligence', 'искусственный интеллект', 'Artificial intelligence is transforming industries.', 'advanced', 'technology'),
('machine learning', 'машинное обучение', 'Machine learning algorithms improve over time.', 'advanced', 'technology'),
('blockchain', 'блокчейн', 'Blockchain technology ensures data security.', 'advanced', 'technology'),
('cybersecurity', 'кибербезопасность', 'Cybersecurity is essential for digital businesses.', 'advanced', 'technology'),
('quantum computing', 'квантовые вычисления', 'Quantum computing will revolutionize computing.', 'advanced', 'technology'),
('augmented reality', 'дополненная реальность', 'Augmented reality enhances user experience.', 'advanced', 'technology'),
('virtual reality', 'виртуальная реальность', 'Virtual reality creates immersive environments.', 'advanced', 'technology'),
('internet of things', 'интернет вещей', 'The Internet of Things connects devices globally.', 'advanced', 'technology'),
('cloud computing', 'облачные вычисления', 'Cloud computing provides scalable infrastructure.', 'advanced', 'technology'),
('big data', 'большие данные', 'Big data analytics reveal valuable insights.', 'advanced', 'technology'),

-- Академические термины
('methodology', 'методология', 'The research methodology was sound.', 'advanced', 'education'),
('hypothesis', 'гипотеза', 'The hypothesis was proven correct.', 'advanced', 'education'),
('empirical', 'эмпирический', 'Empirical evidence supports the theory.', 'advanced', 'education'),
('correlation', 'корреляция', 'There is a strong correlation between variables.', 'advanced', 'education'),
('causation', 'причинность', 'Correlation does not imply causation.', 'advanced', 'education'),
('paradigm', 'парадигма', 'The scientific paradigm has shifted.', 'advanced', 'education'),
('epistemology', 'эпистемология', 'Epistemology studies the nature of knowledge.', 'advanced', 'education'),
('ontology', 'онтология', 'Ontology deals with the nature of being.', 'advanced', 'education'),
('hermeneutics', 'герменевтика', 'Hermeneutics is the study of interpretation.', 'advanced', 'education'),
('phenomenology', 'феноменология', 'Phenomenology examines conscious experience.', 'advanced', 'education'),

-- Медицинские термины
('immunology', 'иммунология', 'Immunology studies the immune system.', 'advanced', 'health'),
('epidemiology', 'эпидемиология', 'Epidemiology tracks disease patterns.', 'advanced', 'health'),
('pathophysiology', 'патофизиология', 'Pathophysiology explains disease mechanisms.', 'advanced', 'health'),
('pharmacology', 'фармакология', 'Pharmacology studies drug effects.', 'advanced', 'health'),
('neurology', 'неврология', 'Neurology deals with nervous system disorders.', 'advanced', 'health'),
('cardiology', 'кардиология', 'Cardiology focuses on heart health.', 'advanced', 'health'),
('oncology', 'онкология', 'Oncology treats cancer patients.', 'advanced', 'health'),
('dermatology', 'дерматология', 'Dermatology treats skin conditions.', 'advanced', 'health'),
('orthopedics', 'ортопедия', 'Orthopedics deals with bone and joint problems.', 'advanced', 'health'),
('radiology', 'радиология', 'Radiology uses imaging for diagnosis.', 'advanced', 'health'),

-- Сложные общие слова
('serendipity', 'счастливая случайность', 'Meeting you here was pure serendipity.', 'advanced', 'general'),
('ubiquitous', 'вездесущий', 'Smartphones are now ubiquitous.', 'advanced', 'general'),
('ephemeral', 'эфемерный', 'Social media posts are often ephemeral.', 'advanced', 'general'),
('resilient', 'устойчивый', 'The community proved to be resilient.', 'advanced', 'general'),
('authentic', 'аутентичный', 'She has an authentic personality.', 'advanced', 'general'),
('profound', 'глубокий', 'The book had a profound impact on me.', 'advanced', 'general'),
('eloquent', 'красноречивый', 'His speech was eloquent and moving.', 'advanced', 'general'),
('persistent', 'настойчивый', 'Success requires persistent effort.', 'advanced', 'general'),
('versatile', 'универсальный', 'This tool is very versatile.', 'advanced', 'general'),
('sophisticated', 'изощренный', 'The solution is quite sophisticated.', 'advanced', 'general')
ON CONFLICT (word, level, category) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

-- Удаление всех карточек
DELETE FROM flashcards WHERE level IN ('beginner', 'intermediate', 'advanced');

-- +goose StatementEnd
