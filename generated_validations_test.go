package // File generated automatically by validator. DO NOT EDIT.
main_test

import "errors"

// Validate implements Validator.
func (e Eqfield) Validate() error {
	if e.Field2 != e.Field1 {
		return errors.New("field \"Field2\" must be equal to \"Field1\"")
	}
	return nil
}

// Validate implements Validator.
func (g Gte) Validate() error {
	if val, than := float64(g.Two), float64(g.One); val < than {
		return errors.New("field \"Two\" must greater or equal than \"One\"")
	}
	return nil
}

// Validate implements Validator.
func (r Required) Validate() error {
	if len(r.String) == 0 {
		return errors.New("field \"String\" is required")
	}
	if r.StringPointer == nil {
		return errors.New("field \"StringPointer\" is required")
	}
	if len(r.Slice) == 0 {
		return errors.New("field \"Slice\" is required")
	}
	if len(r.Map) == 0 {
		return errors.New("field \"Map\" is required")
	}
	return nil
}
