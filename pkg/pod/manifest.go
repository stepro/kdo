package pod

type object map[string]interface{}

func (o object) str(k string) string {
	v := map[string]interface{}(o)[k]
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

func (o object) each(k string, fn func(o object)) object {
	v := o[k]
	if v != nil {
		for _, elem := range v.([]interface{}) {
			fn(elem.(map[string]interface{}))
		}
	}

	return o
}

func (o object) withelem(k string, name string, fn func(o object)) object {
	v := o[k]
	if v != nil {
		for _, elem := range v.([]interface{}) {
			obj := elem.(map[string]interface{})
			if obj["name"] == name {
				fn(obj)
				return o
			}
		}
	}

	elem := map[string]interface{}{
		"name": name,
	}
	o.append(k, elem)
	fn(elem)

	return o
}

func (o object) set(from object, k ...string) object {
	for _, k := range k {
		v := from[k]
		if v == nil {
			continue
		}

		o[k] = v
	}

	return o
}

func (o object) append(k string, elem object) object {
	v := o[k]
	if v == nil {
		v = []interface{}{}
	}

	o[k] = append(v.([]interface{}), elem)

	return o
}
