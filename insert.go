package eywa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type insertOne[T any, M Model[T]] struct {
	*querySkeleton[T, M]
	object M
}

func InsertOne[T any, M Model[T]](object M) *insertOne[T, M] {
	return &insertOne[T, M]{
		querySkeleton: &querySkeleton[T, M]{
			operationName: "insertOne",
		},
		object: object,
	}
}

func (io *insertOne[T, M]) Select(fields ...string) *insertOne[T, M] {
	io.setSelectFields(fields)
	return io
}

func (io *insertOne[T, M]) build() (string, error) {
	obj, err := encodeModel(io.object)
	if err != nil {
		return "", err
	}
	q := fmt.Sprintf(
		"mutation %s {insert_%s_one(object: %s) {%s}}",
		io.querySkeleton.operationName,
		io.object.ModelName(),
		string(obj),
		strings.Join(io.querySkeleton.selectFields, "\n"),
	)
	return q, nil
}

func (io *insertOne[T, M]) Exec(client *Client) (M, error) {
	query, err := io.build()
	if err != nil {
		return nil, fmt.Errorf("couldn't build query: %w", err)
	}
	respBytes, err := client.do(query)
	if err != nil {
		return nil, fmt.Errorf("client query failed: %w", err)
	}
	type gqlResp struct {
		Data   map[string]M   `json:"data"`
		Errors []graphqlError `json:"errors"`
	}

	var respObj gqlResp
	err = json.NewDecoder(respBytes).Decode(&respObj)
	if err != nil {
		return nil, err
	}
	return respObj.Data[fmt.Sprintf("insert_%s_one", io.object.ModelName())], nil
}

type insert[T any, M Model[T]] struct {
	*querySkeleton[T, M]

	affectedRows bool
	objects      modelArr[T, M]
}

func Insert[T any, M Model[T]](object M, objects ...M) *insert[T, M] {
	return &insert[T, M]{
		querySkeleton: &querySkeleton[T, M]{
			operationName: "insert",
		},
		affectedRows: false,
		objects:      append(objects, object),
	}
}

func (iq *insert[T, M]) Select(fields ...string) *insert[T, M] {
	iq.setSelectFields(fields)
	return iq
}

func (iq *insert[T, M]) AffectedRows() *insert[T, M] {
	iq.affectedRows = true
	return iq
}

func (iq *insert[T, M]) build() (string, error) {
	insertQueryFormat := "mutation %s {insert_%s(objects: %s){%s}}"
	objs, err := marshal(iq.objects)
	if err != nil {
		return "", err
	}
	returnBlock := ""
	if len(iq.querySkeleton.selectFields) != 0 {
		returnBlock = fmt.Sprintf("returning{%s}", strings.Join(iq.querySkeleton.selectFields, "\n"))
	}
	if iq.affectedRows || returnBlock == "" {
		returnBlock = "affected_rows\n" + returnBlock
	}

	q := fmt.Sprintf(
		insertQueryFormat,
		iq.operationName,
		iq.objects[0].ModelName(),
		string(objs),
		returnBlock,
	)
	return q, nil
}

type InsertResponse[T any, M Model[T]] struct {
	AffectedRows *int `json:"affected_rows"`
	Returning    []M  `json:"returning"`
}

func (iq *insert[T, M]) Exec(client *Client) (*InsertResponse[T, M], error) {
	query, err := iq.build()
	if err != nil {
		return nil, fmt.Errorf("couldn't build query: %w", err)
	}
	respBytes, err := client.do(query)
	if err != nil {
		return nil, fmt.Errorf("client query failed: %w", err)
	}
	type gqlResp struct {
		Data   map[string]*InsertResponse[T, M] `json:"data"`
		Errors []graphqlError                   `json:"errors"`
	}

	var respObj gqlResp
	err = json.NewDecoder(respBytes).Decode(&respObj)
	if err != nil {
		return nil, err
	}
	return respObj.Data[fmt.Sprintf("insert_%s", iq.objects[0].ModelName())], nil
}

type modelRawJsonMap map[string]json.RawMessage

func (m modelRawJsonMap) marshalGQL() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	buf := bytes.NewBufferString("{")
	i := 0
	for k, v := range m {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(k)
		buf.WriteString(": ")
		buf.Write(v)
		i++
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

type modelArr[T any, M Model[T]] []M

func (ma modelArr[T, M]) marshalGQL() ([]byte, error) {
	if ma == nil {
		return []byte("null"), nil
	}
	buf := bytes.NewBufferString("[")
	for i, m := range ma {
		if i > 0 {
			buf.WriteString(", ")
		}
		modelBytes, err := encodeModel(m)
		if err != nil {
			return nil, err
		}
		buf.Write(modelBytes)
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func encodeModel[T any, M Model[T]](model M) ([]byte, error) {
	if model == nil {
		return []byte("null"), nil
	}
	modelBytes, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}
	var modelMap modelRawJsonMap
	if err != json.Unmarshal(modelBytes, &modelMap) {
		return nil, err
	}
	return marshal(modelMap)
}

type marshaller interface {
	marshalGQL() ([]byte, error)
}

func marshal(m marshaller) ([]byte, error) {
	return m.marshalGQL()
}

//
//var modelInterfaceType *reflect.Type = reflect.TypeOf(new(Model))
//
//func encodeModelValue(value *reflect.Value) ([]byte, error) {
//	if value.IsNil() {
//		return []byte{"null"}
//	}
//	buf := bytes.NewBuffer("{")
//	valType := value.Type()
//	for i := 0; i < value.NumField(); i++ {
//		if i > 0 {
//			buf.WriteByte(',')
//		}
//		tag :=
//
//	}
//}
//
//func encodeValue(value *reflect.Value) ([]byte, error) {
//	if value.Kind() >= reflect.Bool && value.Kind() <= value.Complex128 || value.Kind() == reflect.String {
//		return json.Marshal(v.Interface())
//	} else if value.Kind() == reflect.Pointer {
//		if value.Type().Implements(modelInterfaceType) {
//		}
//		return encodeValue(value.Elem())
//	} else if value.Kind() == reflect.Struct {
//		return json.Marshal(v.Interface())
//	} else if value.Kind() == reflect.Interface {
//		return encodeValue(value.Elem())
//	} else if value.Kind() == reflect.Map {
//		return json.Marshal(v.Interface())
////		bytes := []byte{"{"}
////		iter := value.Range()
////		for iter.Next() {
////			keyBytes, err := encodeValue(iter.Key())
////			if err != nil {
////				return nil, err
////			}
////			valBytes, err := encodeValue(iter.Value())
////			if err != nil {
////				return nil, err
////			}
////			bytes = append(bytes, keyBytes...)
////			bytes = append(bytes, valBytes...)
////		}
////		bytes = append(bytes, byte("}"))
//	} else if value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
//		buf := bytes.NewBuffer("[")
//		for i := 0; i < value.Len(); i++ {
//			if i > 0 {