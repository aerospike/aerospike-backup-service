package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
)

type jsonObject struct {
	keys   []string
	values map[string]any
}

func decodeJSONObject(dec *json.Decoder) (*jsonObject, error) {
	obj := &jsonObject{values: make(map[string]any)}
	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}

		key, ok := token.(string)
		if !ok {
			return nil, fmt.Errorf("expected object key, got %T", token)
		}

		value, err := decodeJSONValue(dec)
		if err != nil {
			return nil, err
		}

		obj.keys = append(obj.keys, key)
		obj.values[key] = value
	}

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	return obj, nil
}

func decodeJSONArray(dec *json.Decoder) ([]any, error) {
	var values []any
	for dec.More() {
		value, err := decodeJSONValue(dec)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}

	if _, err := dec.Token(); err != nil {
		return nil, err
	}

	return values, nil
}

func decodeJSONValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}

	switch value := token.(type) {
	case json.Delim:
		switch value {
		case '{':
			return decodeJSONObject(dec)
		case '[':
			return decodeJSONArray(dec)
		default:
			return nil, fmt.Errorf("unexpected delimiter %q", value)
		}
	default:
		return value, nil
	}
}

func parseJSONObject(data []byte) (*jsonObject, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}

	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, errors.New("expected JSON object")
	}

	return decodeJSONObject(dec)
}

func (o *jsonObject) MarshalJSON() ([]byte, error) {
	return marshalJSONValue(o)
}

func marshalJSONValue(value any) ([]byte, error) {
	switch typed := value.(type) {
	case *jsonObject:
		var buf bytes.Buffer
		buf.WriteByte('{')
		for i, key := range typed.keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			keyBytes, err := marshalJSONLiteral(key)
			if err != nil {
				return nil, err
			}
			buf.Write(keyBytes)
			buf.WriteByte(':')

			valueBytes, err := marshalJSONValue(typed.values[key])
			if err != nil {
				return nil, err
			}
			buf.Write(valueBytes)
		}
		buf.WriteByte('}')

		return buf.Bytes(), nil
	case []any:
		var buf bytes.Buffer
		buf.WriteByte('[')
		for i, item := range typed {
			if i > 0 {
				buf.WriteByte(',')
			}
			itemBytes, err := marshalJSONValue(item)
			if err != nil {
				return nil, err
			}
			buf.Write(itemBytes)
		}
		buf.WriteByte(']')

		return buf.Bytes(), nil
	default:
		return marshalJSONLiteral(value)
	}
}

func marshalJSONLiteral(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}

	b := buf.Bytes()
	if len(b) > 0 && b[len(b)-1] == '\n' {
		b = b[:len(b)-1]
	}

	return b, nil
}

func marshalJSONObjectIndent(value any, indent string) ([]byte, error) {
	compact, err := marshalJSONValue(value)
	if err != nil {
		return nil, err
	}

	var indented bytes.Buffer
	if err := json.Indent(&indented, compact, "", indent); err != nil {
		return nil, err
	}
	indented.WriteByte('\n')

	return indented.Bytes(), nil
}

func asJSONObject(value any) (*jsonObject, bool) {
	obj, ok := value.(*jsonObject)
	return obj, ok
}

func (o *jsonObject) clone() *jsonObject {
	cloned := &jsonObject{
		keys:   append([]string(nil), o.keys...),
		values: make(map[string]any, len(o.values)),
	}
	maps.Copy(cloned.values, o.values)

	return cloned
}

func (o *jsonObject) keyIndex(key string) int {
	for i, existing := range o.keys {
		if existing == key {
			return i
		}
	}
	return -1
}

func (o *jsonObject) removeKey(key string) {
	index := o.keyIndex(key)
	if index < 0 {
		return
	}

	o.keys = append(o.keys[:index], o.keys[index+1:]...)
	delete(o.values, key)
}

func (o *jsonObject) insertKeyAt(index int, key string, value any) {
	o.keys = append(o.keys[:index], append([]string{key}, o.keys[index:]...)...)
	o.values[key] = value
}
