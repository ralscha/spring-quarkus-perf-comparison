package fruit

type AddressDTO struct {
	Address string `json:"address,omitempty"`
	City    string `json:"city,omitempty"`
	Country string `json:"country,omitempty"`
}

type StoreDTO struct {
	ID       int64       `json:"id,omitempty"`
	Name     string      `json:"name,omitempty"`
	Currency string      `json:"currency,omitempty"`
	Address  *AddressDTO `json:"address,omitempty"`
}

type StoreFruitPriceDTO struct {
	Store *StoreDTO `json:"store,omitempty"`
	Price float64   `json:"price"`
}

type FruitDTO struct {
	ID          int64                `json:"id,omitempty"`
	Name        string               `json:"name,omitempty"`
	Description string               `json:"description,omitempty"`
	StorePrices []StoreFruitPriceDTO `json:"storePrices,omitempty"`
}
