package models

type KitabTitle struct {
	Awalan    uint   `json:"Awalan"`
	NKitab    string `json:"NKitab"`
	NKitabEng string `json:"NKitabEng"`
	NKitabUrd string `json:"NKitabUrd"`
	NKitabBen string `json:"NKitabBen"`
	VMember   uint   `json:"VMember"`
}
