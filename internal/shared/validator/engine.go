package validator

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	en_translations "github.com/go-playground/validator/v10/translations/en"
	"github.com/shopspring/decimal"

	"github.com/haditssoft/haditssoft-backend/internal/shared/auth"
	"github.com/haditssoft/haditssoft-backend/internal/shared/database"
	"github.com/haditssoft/haditssoft-backend/internal/shared/utils"
)

type trCtxKey string
type validationError string

var TrCtx trCtxKey = "db-transaction"
var ErrorWhenValidate validationError = "error when validate"

var uni *ut.UniversalTranslator
var validate = validator.New()

func parseJsonName(fld reflect.StructField) string {
	name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
	if name == "-" {
		return ""
	}
	return name
}

func reflectValueToString(value reflect.Value) (string, error) {
	if !value.IsValid() {
		return "", errors.New("zero value")
	}
	var newValue string
	newValType := value.Interface()
	switch value.Interface().(type) {
	case string:
		newValue = newValType.(string)
	case int:
		newValue = strconv.Itoa(newValType.(int))
	case int8:
		newValue = strconv.Itoa(int(newValType.(int8)))
	case int16:
		newValue = strconv.Itoa(int(newValType.(int16)))
	case int32:
		newValue = strconv.Itoa(int(newValType.(int32)))
	case int64:
		newValue = strconv.Itoa(int(newValType.(int64)))
	case uint:
		newValue = strconv.Itoa(int(newValType.(uint)))
	case uint8:
		newValue = strconv.Itoa(int(newValType.(uint8)))
	case bool:
		if newValType.(bool) {
			newValue = "1"
		} else {
			newValue = "0"
		}
	case *bool:
		newValTypex := value.Elem().Interface()
		if newValTypex.(bool) {
			newValue = "1"
		} else {
			newValue = "0"
		}
	case decimal.Decimal:
		newValue = newValType.(decimal.Decimal).String()
	default:
		return "", errors.New("out of coverage type")
	}
	return newValue, nil
}

func isPasswordValid(fl validator.FieldLevel) bool {
	param := fl.Param()
	params := strings.Split(param, " ")
	this := fl.Parent()
	targetId := this.FieldByName(strings.ToUpper(params[1]))
	newValue := fl.Field()

	result := map[string]interface{}{}
	if err := database.DB.Table(params[0]).Select(params[2]).Take(&result, params[1]+" = ?", fmt.Sprintf("%d", targetId.Uint())).Error; err != nil {
		return false
	}

	passwordVal, ok := result["Password"].(string)
	if !ok || !auth.CheckPasswordHash(newValue.Interface().(string), passwordVal) {
		return false
	}

	return true
}

func isExistsDBDynamicTable(fl validator.FieldLevel) bool {
	param := fl.Param()
	params := strings.Split(param, " ")
	if len(params) != 2 {
		return false
	}

	this := fl.Parent()
	tableName := this.FieldByName(params[0]).String()
	columnName := params[1]

	newValue, err := reflectValueToString(fl.Field())
	if err != nil {
		return false
	}

	container := map[string]interface{}{}

	result := database.DB.Table(tableName).
		Select(columnName).
		Where(columnName+" = ?", newValue).
		Take(&container)

	return result.Error == nil && result.RowsAffected == 1
}

func isExistsDBDynamicTableForNote(fl validator.FieldLevel) bool {
	param := fl.Param()
	params := strings.Split(param, " ")
	if len(params) != 2 {
		return false
	}

	this := fl.Parent()
	tableName := this.FieldByName(params[0]).String()
	columnName := params[1]

	newValue, err := reflectValueToString(fl.Field())
	if err != nil {
		return false
	}

	container := map[string]interface{}{}
	query := database.DB.Table(tableName+"Note").
		Select("id").
		Where(columnName+" = ?", newValue).
		Where("deleted_at IS NULL")
	if userID := database.DB.Statement.Context.Value("user_id"); userID != nil {
		query = query.Where("user_id = ?", userID)
	}

	result := query.Take(&container)
	return result.Error == nil && result.RowsAffected == 1
}

func isNotExistsDBDynamicTableForNote(fl validator.FieldLevel) bool {
	return !isExistsDBDynamicTableForNote(fl)
}

func isExistsDB(fl validator.FieldLevel) bool {
	param := fl.Param()
	params := strings.Split(param, " ")

	newValue, err := reflectValueToString(fl.Field())
	if err != nil {
		return false
	}

	container := map[string]interface{}{}

	result := database.DB.Table(
		params[0],
	).Select(
		"id",
	).Where(
		params[1]+" = ?", newValue,
	).Where(
		"DeletedAt IS NULL",
	).Take(&container)
	if result.Error == nil && result.RowsAffected == 1 {
		return true
	}
	return false
}

func RegisterCustomValidations() (err error) {

	if err = validate.RegisterValidation("is_password_valid", isPasswordValid); err != nil {
		return
	}
	if err = validate.RegisterValidation("is_exists_db", isExistsDB); err != nil {
		return
	}
	if err = validate.RegisterValidation("is_exists_db_dynamic_table", isExistsDBDynamicTable); err != nil {
		return
	}
	if err = validate.RegisterValidation("is_exists_db_dynamic_table_for_note", isExistsDBDynamicTableForNote); err != nil {
		return
	}
	if err = validate.RegisterValidation("is_not_exists_db_dynamic_table_for_note", isNotExistsDBDynamicTableForNote); err != nil {
		return
	}

	validate.RegisterTagNameFunc(parseJsonName)
	return
}

func ValidateModel(modelStruct interface{}, ctx ...context.Context) map[string]interface{} {
	en := en.New()
	uni = ut.New(en, en)

	trans, _ := uni.GetTranslator("en")

	en_translations.RegisterDefaultTranslations(validate, trans)

	var allErrors = make(map[string]interface{})
	var err error
	if len(ctx) > 0 {
		err = validate.StructCtx(ctx[0], modelStruct)
	} else {
		xtrex := context.WithValue(context.Background(), TrCtx, database.DB)
		err = validate.StructCtx(xtrex, modelStruct)
	}
	if err != nil {
		subSlice := make(map[string]interface{})
		subSliceItems := make(map[string][]interface{})

		errs := err.(validator.ValidationErrors)

		for _, e := range errs {

			lower := strings.ToLower(e.Field())

			subs := split(e.Namespace())
			if len(subs) == 3 {

				var before string
				var after string
				var found bool
				for i := (len(subs) - 2); i > 0; i-- {
					sub := subs[i]
					before, after, found = strings.Cut(sub, "[")
					if found {
						errLen := len(subSliceItems[before])
						if errLen == 0 {
							subSliceItems[before] = []interface{}{}
						}
						idx, _, gotcha := strings.Cut(after, "]")
						if gotcha {
							num, err := strconv.Atoi(idx)
							if err == nil {
								if errLen != 0 {
									num = num - errLen
								}
								for ix := 0; ix < num; ix++ {
									subSliceItems[before] = append(subSliceItems[before], map[string]interface{}{})
								}
							}
						}
					} else {
						continue
					}
				}
				subSlice[lower] = errorMessage(e, trans)
				subSliceItems[before] = append(subSliceItems[before], subSlice)
				continue
			} else if len(subs) > 3 {
				errorConstructor(subs, 1, subSlice, e, trans)

				return subSlice
			}

			allErrors[lower] = errorMessage(e, trans)
		}
		for k, v := range subSliceItems {
			allErrors[k] = v
		}
	}
	return allErrors
}

func errorMessage(e validator.FieldError, trans ut.Translator) (s string) {
	switch tag := e.ActualTag(); tag {
	case "is_password_valid":
		s = "Value doesn't match our record"
	case "is_exists_db":
		s = "Record does not exists"
	case "is_exists_db_dynamic_table":
		s = "Record does not existstttttttttttttt"
	case "is_exists_db_dynamic_table_for_note":
		s = "Record does not existsosssss"
	case "is_not_exists_db_dynamic_table_for_note":
		s = "Record already exists"
	default:
		s = e.Translate(trans)
	}
	return
}

func split(str string) []string {
	return strings.Split(str, ".")
}

func errorConstructor(subs []string, startIndex int, subSlice map[string]interface{}, e validator.FieldError, trans ut.Translator) map[string]interface{} {
	if startIndex > len(subs)-1 {
		errMsg := make(map[string]interface{})
		errMsg["onlyvalue"] = errorMessage(e, trans)
		return errMsg
	}
	for index, sub := range subs {
		if index < startIndex {
			continue
		}
		ky, num, isSlice := utils.GetKeyAndIndex(sub)
		if isSlice {
			ln := num + 1
			if subSlice[ky] == nil {
				subSlice[ky] = []map[string]interface{}{{}}
			}
			for ix := 0; ix < ln; ix++ {
				if ix == num {
					subSlice[ky].([]map[string]interface{})[ix] = errorConstructor(subs, startIndex+1, subSlice, e, trans)
				} else {
					subSlice[ky] = append(subSlice[ky].([]map[string]interface{}), map[string]interface{}{})
				}
			}
			return subSlice
		} else {
			if startIndex > 1 {
				temp := errorConstructor(subs, startIndex+1, subSlice, e, trans)
				child := make(map[string]interface{})
				if temp["onlyvalue"] != nil {
					child[ky] = temp["onlyvalue"]
				} else {
					child[ky] = temp
				}
				return child
			} else {
				temp := errorConstructor(subs, startIndex+1, subSlice, e, trans)
				if temp["onlyvalue"] != nil {
					subSlice[ky] = temp["onlyvalue"]
				} else {
					subSlice[ky] = temp
				}

			}
		}
	}
	return nil
}
