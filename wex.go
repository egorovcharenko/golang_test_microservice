package main

import (
	"io/ioutil"
	"encoding/json"
	"fmt"
	"time"
	"net/http"
	"strings"
)

// срока содержащая все пары для запроса
var pairString string

// структура для парсинга ответа сервера
type serverInfoResponse struct {
	ServerTime int                    `json:"server_time"`
	Pairs      map[string]interface{} `json:"pairs"`
}

// свой клиент для установки таймаута
var wexClient = &http.Client{Timeout: 5 * time.Second}

func normailzeSymbol(symbol string) (returned string) {
	returned = strings.ToUpper(strings.Replace(symbol, "_", "", 1))
	return
}

func populatePairsWex() {
	// Получить с сервера все возможные пары
	r, err := wexClient.Get(wexExchange.apiUrl + "info")
	if err != nil {
		panic(err)
	}
	defer r.Body.Close()

	// Прочитать JSON ответ
	m := serverInfoResponse{}
	bodyBytes, _ := ioutil.ReadAll(r.Body)
	err = json.Unmarshal(bodyBytes, &m)
	if err != nil {
		panic(err)
	}

	// заполнить массив пар
	pairString = ""
	wexExchange.pairs = make(map[string]Pair, len(m.Pairs))
	i := 0
	for key := range m.Pairs {
		wexExchange.pairs[normailzeSymbol(key)] = Pair{}
		if i > 0 {
			pairString += "-" + key
		} else {
			pairString += key
		}
		i ++
	}

	// Начать заполнять пары
	go pollPairsWex()
}

func pollPairsWex() {
	for {
		// Получить с сервера котировки пар
		r, err := wexClient.Get(wexExchange.apiUrl + "ticker/" + pairString)
		if err != nil {
			panic(err)
		}
		// Прочитать JSON ответ
		var m map[string]map[string]float64
		bodyBytes, _ := ioutil.ReadAll(r.Body)

		// закрываем соединение
		r.Body.Close()

		err = json.Unmarshal(bodyBytes, &m)
		if err != nil {
			panic(err)
		}

		// обновить котировки в базе
		for key, value := range m {
			key = normailzeSymbol(key)
			wexExchange.pairsMutex.RLock()
			pair, ok := wexExchange.pairs[key]
			wexExchange.pairsMutex.RUnlock()
			if !ok {
				fmt.Printf("котировки %v не найдено\n", key)
				break
			}
			price := Number(value["last"])
			pair.allPrices = append(pair.allPrices, PriceAtMoment{price, time.Now()})

			// вычисляем и заменяем среднее за 10 минут
			if len(pair.allPrices) > 0 {
				sum, num := Number(0.0), 0
				newPrices := make([]PriceAtMoment, 0)
				for _, p := range pair.allPrices {
					if time.Since(p.Time).Minutes() < 10 {
						sum += p.Price
						num ++
						// удаляем старые цены из памяти
						newPrices = append(newPrices, p)
					}
				}
				pair.allPrices = newPrices
				pair.AvgP = Number(sum / Number(num))
			}

			// кладем обратно пару в хранилище
			wexExchange.pairsMutex.Lock()
			wexExchange.pairs[key] = pair
			wexExchange.pairsMutex.Unlock()
		}

		// ждем следующего опроса
		time.Sleep(2 * time.Second)
	}
}
