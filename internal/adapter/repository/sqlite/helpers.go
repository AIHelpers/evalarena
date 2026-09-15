package sqlite

import (
	"database/sql"
	"encoding/json"
	"evalarena/internal/domain"
	"reflect"
)

func marshalIf(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map:
		if rv.Len() == 0 {
			return nil, nil
		}
	}
	return json.Marshal(v)
}

func marshalVote(v *domain.Vote) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func unmarshalJSON(n sql.NullString, dst any) error {
	if !n.Valid || n.String == "" {
		return nil
	}
	return json.Unmarshal([]byte(n.String), dst)
}
