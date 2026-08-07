//go:build exclude

package field

type List struct {
	entries []Field
}

// bsearch searches array for value.
func (fields List) bsearch(name string) (index int, found bool) {
	i, j := 0, len(fields.entries)
	for i < j {
		h := i + (j-i)/2
		if name >= fields.entries[h].name {
			i = h + 1
		} else {
			j = h
		}
	}
	if i > 0 && fields.entries[i-1].name >= name {
		return i - 1, true
	}
	return i, false
}

func (fields List) Set(field Field) List {
	var updated List
	index, found := fields.bsearch(field.name)
	if found {
		if field.value.IsZero() {
			// delete
			if len(fields.entries) > 1 {
				updated.entries = make([]Field, len(fields.entries)-1)
				copy(updated.entries, fields.entries[:index])
				copy(updated.entries[index:], fields.entries[index+1:])
			}
		} else if !fields.entries[index].value.Equals(field.value) {
			// update
			updated.entries = make([]Field, len(fields.entries))
			copy(updated.entries, fields.entries)
			updated.entries[index].value = field.value
		} else {
			// nothing changes
			updated = fields
		}
		return updated
	}
	if field.Value().IsZero() {
		return fields
	}
	updated.entries = make([]Field, len(fields.entries)+1)
	copy(updated.entries, fields.entries[:index])
	copy(updated.entries[index+1:], fields.entries[index:])
	updated.entries[index] = field
	return updated
}

func (fields List) Get(name string) Field {
	index, found := fields.bsearch(name)
	if !found {
		return ZeroField
	}
	return fields.entries[index]
}

func (fields List) Scan(iter func(field Field) bool) {
	for _, f := range fields.entries {
		if !iter(f) {
			return
		}
	}
}

func (fields List) Len() int {
	return len(fields.entries)
}

func (fields List) Weight() int {
	var weight int
	for _, f := range fields.entries {
		weight += f.Weight()
	}
	return weight
}
