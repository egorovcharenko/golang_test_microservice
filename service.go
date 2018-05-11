package main

import (
	"time"
	"fmt"
	"sync"
	"net/http"
	"encoding/json"
	"github.com/gorilla/mux"
	"log"
)

// Цена на момент времени
type PriceAtMoment struct {
	Price Number
	Time  time.Time
}

// Новый тип для сериализации с заданным количеством знаков после запятой
type Number float64

func (n Number) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.4f", n)), nil
}

// Пара валют со всеми ценами за 10 минут и средним
type Pair struct {
	AvgP      Number
	allPrices []PriceAtMoment
}

// Все данные, относящиеся к бирже
type Exchange struct {
	pairs  map[string]Pair
	apiUrl string
	// мьютекс для доступа к данным
	pairsMutex sync.RWMutex
}

var wexExchange = Exchange{
	apiUrl: "https://wex.nz/api/3/",
	pairs:  make(map[string]Pair),
}

var exchanges = map[string]*Exchange{
	"wex":     &wexExchange,
	"binance": &binanceExchange,
}

type singlePriceResponse struct {
	ExchangeName string `json:"exchange"`
	AvgP         Number `json:"averagePrice"`
}
type allPricesResponse map[string][]singlePriceResponse

func main() {
	// Начинаем заполнять пары с биржы в память
	go populatePairsWex()
	go pollPairsBinance()

	// Поднимаем веб-сервер для обслуживания API
	rtr := mux.NewRouter()
	rtr.HandleFunc("/ticker/{pair}", getSinglePairHandler).Methods("GET")
	rtr.HandleFunc("/", getAllPairsHandler).Methods("GET")
	http.Handle("/", rtr)
	log.Println("Listening...")
	http.ListenAndServe(":3000", nil)
}

func getSinglePairHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	var response = make([]singlePriceResponse, 0, 2)
	for exchangeName, exchangeData := range exchanges {
		exchangeData.pairsMutex.RLock()
		pair, ok := exchangeData.pairs[params["pair"]]
		exchangeData.pairsMutex.RUnlock()
		if ok {
			response = append(response, singlePriceResponse{
				ExchangeName: exchangeName,
				AvgP:         pair.AvgP,
			})
		}
	}
	jsonInfo, _ := json.Marshal(response)
	w.Write([]byte(jsonInfo))
}

func getAllPairsHandler(w http.ResponseWriter, _ *http.Request) {
	var response = make(allPricesResponse)
	for exchangeName, exchangeData := range exchanges {
		exchangeData.pairsMutex.RLock()
		for symbol, pairData := range exchangeData.pairs {
			existingResponse, ok := response[symbol]
			fmt.Printf("ok: %v, symbol: %v\n", ok, symbol)
			if ok {
				// пара есть в ответе - добавить стоимость на данной бирже просто
				response[symbol] = append(existingResponse, singlePriceResponse{
					exchangeName, pairData.AvgP,
				})

			} else {
				// пары в ответе еще нет - добавить ее
				response[symbol] = []singlePriceResponse{
					{
						exchangeName, pairData.AvgP,
					},
				}
			}
		}
		exchangeData.pairsMutex.RUnlock()
	}
	jsonInfo, _ := json.MarshalIndent(response, " ", " ")
	w.Write([]byte(jsonInfo))
}
