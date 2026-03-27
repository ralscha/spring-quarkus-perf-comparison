package app

import (
	"fmt"
	"strconv"

	"github.com/quarkusio/spring-quarkus-perf-comparison/go/internal/fruit"
)

func marshalJSON(payload any) ([]byte, error) {
	switch value := payload.(type) {
	case []fruit.FruitDTO:
		return appendFruitArrayJSON(make([]byte, 0, 1024), value), nil
	case fruit.FruitDTO:
		return appendFruitJSON(make([]byte, 0, 256), value), nil
	case *fruit.FruitDTO:
		if value == nil {
			return []byte("null"), nil
		}
		return appendFruitJSON(make([]byte, 0, 256), *value), nil
	case errorResponse:
		return appendErrorJSON(make([]byte, 0, 64), value), nil
	default:
		return nil, fmt.Errorf("unsupported JSON payload type %T", payload)
	}
}

func appendFruitArrayJSON(buffer []byte, fruits []fruit.FruitDTO) []byte {
	buffer = append(buffer, '[')
	for index, item := range fruits {
		if index > 0 {
			buffer = append(buffer, ',')
		}
		buffer = appendFruitJSON(buffer, item)
	}
	buffer = append(buffer, ']')
	return buffer
}

func appendFruitJSON(buffer []byte, item fruit.FruitDTO) []byte {
	buffer = append(buffer, '{')
	fieldCount := 0

	if item.ID != 0 {
		buffer = appendJSONFieldName(buffer, fieldCount, "id")
		buffer = strconv.AppendInt(buffer, item.ID, 10)
		fieldCount++
	}
	if item.Name != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "name")
		buffer = strconv.AppendQuote(buffer, item.Name)
		fieldCount++
	}
	if item.Description != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "description")
		buffer = strconv.AppendQuote(buffer, item.Description)
		fieldCount++
	}
	if len(item.StorePrices) > 0 {
		buffer = appendJSONFieldName(buffer, fieldCount, "storePrices")
		buffer = appendStorePriceArrayJSON(buffer, item.StorePrices)
	}

	buffer = append(buffer, '}')
	return buffer
}

func appendStorePriceArrayJSON(buffer []byte, prices []fruit.StoreFruitPriceDTO) []byte {
	buffer = append(buffer, '[')
	for index, item := range prices {
		if index > 0 {
			buffer = append(buffer, ',')
		}
		buffer = appendStorePriceJSON(buffer, item)
	}
	buffer = append(buffer, ']')
	return buffer
}

func appendStorePriceJSON(buffer []byte, item fruit.StoreFruitPriceDTO) []byte {
	buffer = append(buffer, '{')
	fieldCount := 0

	if item.Store != nil {
		buffer = appendJSONFieldName(buffer, fieldCount, "store")
		buffer = appendStoreJSON(buffer, *item.Store)
		fieldCount++
	}

	buffer = appendJSONFieldName(buffer, fieldCount, "price")
	buffer = strconv.AppendFloat(buffer, item.Price, 'g', -1, 64)
	buffer = append(buffer, '}')
	return buffer
}

func appendStoreJSON(buffer []byte, item fruit.StoreDTO) []byte {
	buffer = append(buffer, '{')
	fieldCount := 0

	if item.ID != 0 {
		buffer = appendJSONFieldName(buffer, fieldCount, "id")
		buffer = strconv.AppendInt(buffer, item.ID, 10)
		fieldCount++
	}
	if item.Name != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "name")
		buffer = strconv.AppendQuote(buffer, item.Name)
		fieldCount++
	}
	if item.Currency != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "currency")
		buffer = strconv.AppendQuote(buffer, item.Currency)
		fieldCount++
	}
	if item.Address != nil {
		buffer = appendJSONFieldName(buffer, fieldCount, "address")
		buffer = appendAddressJSON(buffer, *item.Address)
	}

	buffer = append(buffer, '}')
	return buffer
}

func appendAddressJSON(buffer []byte, item fruit.AddressDTO) []byte {
	buffer = append(buffer, '{')
	fieldCount := 0

	if item.Address != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "address")
		buffer = strconv.AppendQuote(buffer, item.Address)
		fieldCount++
	}
	if item.City != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "city")
		buffer = strconv.AppendQuote(buffer, item.City)
		fieldCount++
	}
	if item.Country != "" {
		buffer = appendJSONFieldName(buffer, fieldCount, "country")
		buffer = strconv.AppendQuote(buffer, item.Country)
	}

	buffer = append(buffer, '}')
	return buffer
}

func appendErrorJSON(buffer []byte, item errorResponse) []byte {
	buffer = append(buffer, '{')
	buffer = append(buffer, '"', 'e', 'r', 'r', 'o', 'r', '"', ':')
	buffer = strconv.AppendQuote(buffer, item.Error)
	buffer = append(buffer, '}')
	return buffer
}

func appendJSONFieldName(buffer []byte, fieldCount int, name string) []byte {
	if fieldCount > 0 {
		buffer = append(buffer, ',')
	}
	buffer = strconv.AppendQuote(buffer, name)
	buffer = append(buffer, ':')
	return buffer
}
