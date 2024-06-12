package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const CAMBIO_LINK string = "https://economia.awesomeapi.com.br/json/last/USD-BRL"

type Cambio struct {
	Usdbrl struct {
		Code       string `json:"code"`
		Codein     string `json:"codein"`
		Name       string `json:"name"`
		High       string `json:"high"`
		Low        string `json:"low"`
		VarBid     string `json:"varBid"`
		PctChange  string `json:"pctChange"`
		Bid        string `json:"bid"`
		Ask        string `json:"ask"`
		Timestamp  string `json:"timestamp"`
		CreateDate string `json:"create_date"`
	} `json:"usdBrl"`
}

type Cotacao struct {
	Valor      string
	CreateDate string
}

func main() {
	http.HandleFunc("/cotacao", BuscaCotacaoHandler)
	http.ListenAndServe(":8080", nil)
}

func BuscaCotacaoHandler(w http.ResponseWriter, r *http.Request) {
	db, err := gorm.Open(sqlite.Open("goexpert.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// Já que não estou utilizando o SQLite, optei por utilizar o gorm para facilitar a criação ta tabela
	db.AutoMigrate(&Cotacao{})

	if r.URL.Path != "/cotacao" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	cambio, err := BuscaCambio()
	if err != nil {
		panic(err)
	}

	// Utilizando SQL puro
	cotacao, err := SalvaCotacao(*cambio)
	if err != nil {
		panic(err)
	}

	// Utilizando gorm
	// cotacao, err := SalvaCotacaoGorm(db, *cambio)
	// if err != nil {
	// 	panic(err)
	// }

	w.Header().Set("Context-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(cotacao)
}

func BuscaCambio() (*Cambio, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*200)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", CAMBIO_LINK, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		ValidaTimeOut(err)
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	var cambio Cambio
	err = json.Unmarshal(body, &cambio)
	if err != nil {
		return nil, err
	}

	return &cambio, nil
}

func SalvaCotacao(cambio Cambio) (*Cotacao, error) {
	cotacao := CambioToCotacao(cambio)

	db, err := sql.Open("sqlite3", "goexpert.db")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	stmt, err := db.Prepare("INSERT INTO cotacaos (valor, create_date) VALUES ($1, $2)")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	_, err = stmt.ExecContext(ctx, cotacao.Valor, cotacao.CreateDate)
	if err != nil {
		ValidaTimeOut(err)
		return nil, err
	}

	return &cotacao, nil
}

func SalvaCotacaoGorm(db *gorm.DB, cambio Cambio) (*Cotacao, error) {
	cotacao := CambioToCotacao(cambio)

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	err := db.WithContext(ctx).Create(&cotacao).Error
	if err != nil {
		return nil, err
	}
	return &cotacao, nil
}

func CambioToCotacao(cambio Cambio) Cotacao {
	return Cotacao{
		Valor:      cambio.Usdbrl.Bid,
		CreateDate: cambio.Usdbrl.CreateDate,
	}
}

func ValidaTimeOut(err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		log.Println("Tempo excedido: ", err)
	}
}
