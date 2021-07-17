package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
)

type AddressSuite struct {
	suite.Suite
	db *gorm.DB
	r  *gin.Engine
}

func closeDB(suite *AddressSuite) {
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	os.Remove("addresses_test.db")
}

func (suite *AddressSuite) DoRequest(method, url string, body io.Reader) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(method, url, body)
	suite.r.ServeHTTP(w, req)
	return w
}

func (suite *AddressSuite) SetupSuite() {
	gin.SetMode(gin.ReleaseMode)
}

func (suite *AddressSuite) SetupTest() {
	closeDB(suite)
	suite.db = initialize_database("addresses_test.db")
	suite.r = setup_endpoints(suite.db, false)
}

func (suite *AddressSuite) TearDownSuite() {
	closeDB(suite)
}

func (suite *AddressSuite) TestExample() {
	w := suite.DoRequest("GET", "/address/12345", nil)

	suite.NotEqual(200, w.Code)
}

func (suite *AddressSuite) TestPost() {
	w := suite.DoRequest("POST", "/address", strings.NewReader(`{"first_name": "Jane", "last_name": "Doe", "phone": "070000000"}`))
	suite.Equal(200, w.Code)

	var response struct {
		ID        int    `json:"id"`
		Success   bool   `json:"success"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	suite.True(response.Success)

	w = suite.DoRequest("GET", fmt.Sprintf("/address/%v", response.ID), nil)
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	suite.True(response.Success)
	suite.Equal(response.FirstName, "Jane")
	suite.Equal(response.LastName, "Doe")
	suite.Equal(response.Phone, "070000000")
}

type AddressResponse struct {
	Success   bool        `json:"success"`
	Next      *string     `json:"next"`
	Addresses []Addresses `json:"addresses"`
}

func (suite *AddressSuite) TestPagination() {
	file, _ := os.Open("names.csv")
	csvreader := csv.NewReader(file)
	first_names := make([]string, 0)
	for {
		record, err := csvreader.Read()
		if err == io.EOF {
			break
		}
		first_names = append(first_names, record[0])
	}

	read_names := make([]string, 0)
	cur_url := "/address"
	for {
		w := suite.DoRequest("GET", cur_url, nil)
		var response AddressResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		for _, address := range response.Addresses {
			read_names = append(read_names, address.FirstName)
		}

		if response.Next == nil {
			break
		} else {
			cur_url = *response.Next
		}
	}
	suite.Equal(first_names, read_names)
}

func (suite *AddressSuite) TestPaginationLimit() {
	var response AddressResponse
	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=1000", nil).Body.Bytes(), &response)
	suite.Equal(len(response.Addresses), 100)

	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=42", nil).Body.Bytes(), &response)
	suite.Equal(len(response.Addresses), 42)

	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=-100", nil).Body.Bytes(), &response)
	suite.Equal(len(response.Addresses), 20)

	w := suite.DoRequest("GET", "/address?limit=asdf", nil)
	suite.NotEqual(200, w.Code)
}

func (suite *AddressSuite) TestSorting() {
	file, _ := os.Open("names.csv")
	csvreader := csv.NewReader(file)
	last_names := make([]string, 0)
	for {
		record, err := csvreader.Read()
		if err == io.EOF {
			break
		}
		last_names = append(last_names, record[1])
	}
	sort.Strings(last_names)

	read_names := make([]string, 0)
	cur_url := "/address?sort=last_name"
	for {
		w := suite.DoRequest("GET", cur_url, nil)
		var response AddressResponse
		_ = json.Unmarshal(w.Body.Bytes(), &response)
		for _, address := range response.Addresses {
			read_names = append(read_names, address.LastName)
		}

		if response.Next == nil {
			break
		} else {
			cur_url = *response.Next
		}
	}
	suite.Equal(last_names, read_names)
}

func (suite *AddressSuite) TestDelete() {
	var response AddressResponse
	_ = json.Unmarshal(suite.DoRequest("GET", "/address?search=Thomp", nil).Body.Bytes(), &response)
	suite.Equal(len(response.Addresses), 1)
	suite.Equal(response.Addresses[0].FirstName, "Angela")
	suite.Equal(response.Addresses[0].LastName, "Thompson")

	w := suite.DoRequest("DELETE", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	suite.Equal(w.Code, 200)

	w = suite.DoRequest("DELETE", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	suite.Equal(w.Code, 404)

	w = suite.DoRequest("GET", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	suite.Equal(w.Code, 404)

	_ = json.Unmarshal(suite.DoRequest("GET", "/address?search=Thomp", nil).Body.Bytes(), &response)
	suite.Equal(len(response.Addresses), 0)
}

func TestAddressSuite(t *testing.T) {
	suite.Run(t, new(AddressSuite))
}
