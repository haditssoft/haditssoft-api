package parser

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/gofiber/fiber/v2"
)

type SqlNullTime struct {
	sql.NullTime
}

func (ct *SqlNullTime) String() string {
	t := time.Time(*&ct.Time).String()
	return t
}

func RegisterParser() {
	var nullTimeConverter = func(value string) reflect.Value {
		sqlNullTime := new(SqlNullTime)
		if len(value) < 1 {
			sqlNullTime.Time, sqlNullTime.Valid = time.Time{}, false
			return reflect.ValueOf("")
		}

		if v, err := time.Parse("2006-01-02", value); err == nil {
			sqlNullTime.Valid = true
			sqlNullTime.Time = v
		} else {
			sqlNullTime.Valid = false
		}
		return reflect.ValueOf(*sqlNullTime)
	}

	parser := fiber.ParserType{
		Customtype: SqlNullTime{},
		Converter:  nullTimeConverter,
	}

	fiber.SetParserDecoder(fiber.ParserConfig{
		IgnoreUnknownKeys: true,
		ParserType:        []fiber.ParserType{parser},
		ZeroEmpty:         true,
	})
}
