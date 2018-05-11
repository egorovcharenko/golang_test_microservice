package main

import (
	"io/ioutil"
	"encoding/json"
	"time"
	"net/http"
)

var binanceExchange = Exchange{
	apiUrl: "https://api.binance.com",
	pairs:  make(map[string]Pair),
}

// структура для парсинга ответа сервера
type binanceTickerPriceResponse struct {
	Symbol string `json:"symbol"`
	Price  Number `json:"price,string"`
}

// свой клиент для установки таймаута
var binanceClient = &http.Client{Timeout: 5 * time.Second}

func pollPairsBinance() {
	for {
		//time.Sleep(5 * time.Second)
		// Получить с сервера все возможные пары
		r, err := binanceClient.Get(binanceExchange.apiUrl + "/api/v3/ticker/price")
		if err != nil {
			panic(err)
		}

		// Прочитать JSON ответ
		var m = make([]binanceTickerPriceResponse, 0)
		bodyBytes, _ := ioutil.ReadAll(r.Body)

		// закрываем соединение
		r.Body.Close()
		err = json.Unmarshal(bodyBytes, &m)
		if err != nil {
			panic(err)
		}

		// заполнить массив пар
		binanceExchange.pairs = make(map[string]Pair, len(m))
		binanceExchange.pairsMutex.RLock()
		for _, pair := range m {
			existingPair, ok := binanceExchange.pairs[pair.Symbol]
			if !ok {
				existingPair = Pair{-1, make([]PriceAtMoment, 0)}
				binanceExchange.pairs[pair.Symbol] = existingPair
			}
			existingPair.allPrices = append(existingPair.allPrices, PriceAtMoment{pair.Price, time.Now()})

			// вычисляем и заменяем среднее за 10 минут
			if len(existingPair.allPrices) > 0 {
				sum, num := Number(0.0), 0
				newPrices := make([]PriceAtMoment, 0)
				for _, p := range existingPair.allPrices {
					if time.Since(p.Time).Minutes() < 10 {
						sum += p.Price
						num ++
						// удаляем старые цены из памяти
						newPrices = append(newPrices, p)
					}
				}
				existingPair.allPrices = newPrices
				existingPair.AvgP = Number(sum / Number(num))
			}
			// кладем обратно пару в хранилище
			binanceExchange.pairs[pair.Symbol] = existingPair

		}
		binanceExchange.pairsMutex.RUnlock()

		// ждем следующего опроса
		time.Sleep(2 * time.Second)
	}
}
