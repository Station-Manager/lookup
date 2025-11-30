package hamnut

import (
	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
	"github.com/goccy/go-json"
	"reflect"
	"strconv"
)

func (s *Service) unmarshalResponse(body []byte) (types.Country, error) {
	const op errors.Op = "hamnut.Service.unmarshalResponse"
	var country types.Country

	var result interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return country, err
	}

	toffset := result.(map[string]interface{})["localTime"].(string)
	country.TimeOffset = toffset[len(toffset)-6:]

	if data, good := result.(map[string]interface{}); good {
		userType := reflect.TypeOf(country)
		countryValue := reflect.ValueOf(&country).Elem()
		for i := 0; i < userType.NumField(); i++ {
			field := userType.Field(i)
			tag := field.Tag.Get("hamnut")
			if _, exists := data[tag]; exists {
				countryField := countryValue.Field(i)
				valueType := reflect.TypeOf(data[tag])
				switch valueType.Kind() {
				case reflect.String:
					if str, ok := data[tag].(string); ok {
						countryField.SetString(str)
					}
				case reflect.Float64:
					if f, ok := data[tag].(float64); ok {
						countryField.SetString(strconv.FormatFloat(f, 'g', -1, 64))
					}
				default:
					return country, errors.New(op).Msgf("unhandled field type: %s", field.Type.Kind())
				}
			}
		}
	} else {
		return country, errors.New(op).Msgf("unexpected type: %T", result)
	}

	return country, nil
}
