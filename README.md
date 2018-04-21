# Тестовый микросервис
показывает среднюю котировку валютной пары за 10 минут
## Использование
* Создаем docker image: `docker build -t microservice-test .`
* Запускаем его (внутри сервер работает на порту 3000): `docker run -p 3000:3000 microservice-test`
* Получаем котировку: `http://localhost:3000/ticker/bch_usd`
