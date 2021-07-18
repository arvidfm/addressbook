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

type SuccessResponse struct {
	Success *bool   `json:"success"`
	Error   *string `json:"error"`
}

func (suite *AddressSuite) TestExample() {
	w := suite.DoRequest("GET", "/address/12345", nil)
	var response SuccessResponse
	_ = json.Unmarshal(w.Body.Bytes(), &response)

	suite.NotEqual(200, w.Code)
	if response.Success != nil {
		suite.False(*response.Success)
	}
	suite.NotNil(response.Error)
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
	suite.Equal("Jane", response.FirstName)
	suite.Equal("Doe", response.LastName)
	suite.Equal("070000000", response.Phone)
}

type AddressResponse struct {
	Success   *bool       `json:"success"`
	Next      *string     `json:"next"`
	Addresses []Addresses `json:"addresses"`
	Error     *string     `json:"error"`
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
	response := AddressResponse{}
	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=1000", nil).Body.Bytes(), &response)
	suite.Equal(100, len(response.Addresses))
	suite.NotNil(response.Success)
	suite.True(*response.Success)

	response = AddressResponse{}
	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=42", nil).Body.Bytes(), &response)
	suite.Equal(42, len(response.Addresses))
	suite.NotNil(response.Success)
	suite.True(*response.Success)

	response = AddressResponse{}
	_ = json.Unmarshal(suite.DoRequest("GET", "/address?limit=-100", nil).Body.Bytes(), &response)
	suite.Equal(20, len(response.Addresses))
	suite.NotNil(response.Success)
	suite.True(*response.Success)

	response = AddressResponse{}
	w := suite.DoRequest("GET", "/address?limit=asdf", nil)
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	suite.NotEqual(200, w.Code)
	if response.Success != nil {
		suite.False(*response.Success)
	}
	suite.NotNil(response.Error)
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
	suite.Equal(1, len(response.Addresses))
	suite.Equal("Angela", response.Addresses[0].FirstName)
	suite.Equal("Thompson", response.Addresses[0].LastName)

	address_response := SuccessResponse{}
	w := suite.DoRequest("DELETE", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	_ = json.Unmarshal(w.Body.Bytes(), &address_response)
	suite.Equal(200, w.Code)
	suite.NotNil(address_response.Success)
	suite.True(*address_response.Success)
	suite.Nil(address_response.Error)

	address_response = SuccessResponse{}
	w = suite.DoRequest("DELETE", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	_ = json.Unmarshal(w.Body.Bytes(), &address_response)
	suite.Equal(404, w.Code)
	if address_response.Success != nil {
		suite.False(*address_response.Success)
	}
	suite.NotNil(address_response.Error)

	address_response = SuccessResponse{}
	w = suite.DoRequest("GET", fmt.Sprintf("/address/%v", response.Addresses[0].ID), nil)
	_ = json.Unmarshal(w.Body.Bytes(), &address_response)
	suite.Equal(404, w.Code)
	if address_response.Success != nil {
		suite.False(*address_response.Success)
	}
	suite.NotNil(address_response.Error)

	_ = json.Unmarshal(suite.DoRequest("GET", "/address?search=Thomp", nil).Body.Bytes(), &response)
	suite.Equal(0, len(response.Addresses))
}

func TestAddressSuite(t *testing.T) {
	suite.Run(t, new(AddressSuite))
}
