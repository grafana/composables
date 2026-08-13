package model

import "regexp"

const (
	domainMsg = "domain must be a lowercase k8s-style slug and not the reserved 'kg'"
	typeMsg   = "type must be a valid identifier"
	blankMsg  = "must not be blank"
	nullMsg   = "must not be null"
)

var (
	domainRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)
	typeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
)

// ValidateDomain returns a FieldError for a domain value, or nil if valid.
// Blank -> "must not be blank"; otherwise must match the slug pattern and not be "kg".
func ValidateDomain(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	if v == "kg" || !domainRe.MatchString(v) {
		return &FieldError{Field: field, RejectedValue: v, HasRejected: true, Message: domainMsg}
	}
	return nil
}

// ValidateType returns a FieldError for a type value, or nil if valid.
func ValidateType(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	if !typeRe.MatchString(v) {
		return &FieldError{Field: field, RejectedValue: v, HasRejected: true, Message: typeMsg}
	}
	return nil
}

func validateBlank(field, v string) *FieldError {
	if v == "" {
		return &FieldError{Field: field, Message: blankMsg}
	}
	return nil
}

// ValidateEntity validates an entity upsert body. Errors are returned in a
// deterministic declaration order (domain, type, name, ttlSeconds).
func ValidateEntity(r EntityWriteRequest) []FieldError {
	var errs []FieldError
	if e := ValidateDomain("domain", r.Domain); e != nil {
		errs = append(errs, *e)
	}
	if e := ValidateType("type", r.Type); e != nil {
		errs = append(errs, *e)
	}
	if e := validateBlank("name", r.Name); e != nil {
		errs = append(errs, *e)
	}
	if r.TTLSeconds == nil {
		errs = append(errs, FieldError{Field: "ttlSeconds", Message: nullMsg})
	}
	return errs
}

// ValidateRelationship validates a relationship upsert body.
// Order: domain, type, from, to, ttlSeconds.
func ValidateRelationship(r RelationshipWriteRequest) []FieldError {
	var errs []FieldError
	if e := ValidateDomain("domain", r.Domain); e != nil {
		errs = append(errs, *e)
	}
	if e := ValidateType("type", r.Type); e != nil {
		errs = append(errs, *e)
	}
	if r.From == nil {
		errs = append(errs, FieldError{Field: "from", Message: nullMsg})
	}
	if r.To == nil {
		errs = append(errs, FieldError{Field: "to", Message: nullMsg})
	}
	if r.TTLSeconds == nil {
		errs = append(errs, FieldError{Field: "ttlSeconds", Message: nullMsg})
	}
	return errs
}
