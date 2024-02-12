package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGETHelloWorld(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	w := httptest.NewRecorder()
	helloHandler(w, req)
	res := w.Result()
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	if string(data) != "Hello World!\n" {
		t.Errorf("expected Hello World got %v", string(data))
	}
}

func TestGetPersonByUsernm(t *testing.T) {
	cris := Person{
		Name: "cris",
		Age:  26,
	}
	db, dbmock, err := sqlmock.New()
	NewDB(db)
	if err != nil {
		t.Errorf("an error '%s' was not expected when opening a stub database connection", err)
	}
	queryTable := "SELECT username, age FROM DEMO.persons WHERE username = :username"
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/person/%s", cris.Name), nil)
	w := httptest.NewRecorder()
	rows := sqlmock.NewRows([]string{"username", "age"}).AddRow(cris.Name, cris.Age)
	dbmock.ExpectQuery(queryTable).WithArgs(cris.Name).WillReturnRows(rows)

	handler(w, req)

	res := w.Result()
	if res.StatusCode != http.StatusOK {
		t.Errorf("expected 200 got %v", res.StatusCode)
	}
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("expected error to be nil got %v", err)
	}
	var result Person
	err = json.Unmarshal(data, &result)
	if err != nil {
		t.Errorf("unmarshal error %v", err.Error())
	}
	if cris != result {
		t.Errorf("expected error to be nil got %v", err)
	}
}
