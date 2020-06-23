package pod

type object map[string]interface{}

func (o object) num(k string) int {
	v := o[k]
	if v == nil {
		return 0
	}

	return int(v.(float64))
}

func (o object) str(k string) string {
	v := o[k]
	if v == nil {
		return ""
	}

	return v.(string)
}

func (o object) obj(k string) object {
	v := o[k]
	if v == nil {
		return nil
	}

	return v.(map[string]interface{})
}

func (o object) with(k string, fn func(o object)) object {
	v := o[k]
	if v == nil {
		v = map[string]interface{}{}
		o[k] = v
	}

	fn(v.(map[string]interface{}))

	return o
}

type array []interface{}

func (o object) arr(k string) array {
	v := o[k]
	if v == nil {
		return nil
	}

	return v.([]interface{})
}

func (o object) appendobj(k string, elem object) object {
	v := o[k]
	if v == nil {
		v = []interface{}{}
	}

	o[k] = append(v.([]interface{}), map[string]interface{}(elem))

	return o
}

func (o object) withelem(k string, name string, fn func(o object)) object {
	v := o[k]
	var obj map[string]interface{}
	if v != nil {
		for _, elem := range v.([]interface{}) {
			obj = elem.(map[string]interface{})
			if obj["name"] == name {
				break
			}
			obj = nil
		}
	}

	if obj == nil {
		obj = map[string]interface{}{
			"name": name,
		}
		o.appendobj(k, obj)
	}

	fn(obj)

	return o
}

func (o object) set(src object, k ...string) object {
	for _, k := range k {
		v := src[k]
		if v == nil {
			continue
		}
		o[k] = v
	}

	return o
}
