package database

import (
	"gorm.io/gorm"
)

func LimitRangeSort(ranges []int, sort []string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if len(ranges) == 2 {
			db.Offset(ranges[0])
			limit := ranges[1] - ranges[0] + 1
			if limit > 0 {
				db.Limit(limit)
			}
		}
		for _, orderStr := range sort {
			db.Order(orderStr)
		}
		return db
	}
}
