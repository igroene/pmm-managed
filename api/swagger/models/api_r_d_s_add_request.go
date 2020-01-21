// Code generated by go-swagger; DO NOT EDIT.

package models

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	strfmt "github.com/go-openapi/strfmt"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/swag"
)

// APIRDSAddRequest api r d s add request
// swagger:model apiRDSAddRequest
type APIRDSAddRequest struct {

	// aws access key id
	AwsAccessKeyID string `json:"aws_access_key_id,omitempty"`

	// aws secret access key
	AwsSecretAccessKey string `json:"aws_secret_access_key,omitempty"`

	// id
	ID *APIRDSInstanceID `json:"id,omitempty"`

	// password
	Password string `json:"password,omitempty"`

	// username
	Username string `json:"username,omitempty"`
}

// Validate validates this api r d s add request
func (m *APIRDSAddRequest) Validate(formats strfmt.Registry) error {
	var res []error

	if err := m.validateID(formats); err != nil {
		res = append(res, err)
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

func (m *APIRDSAddRequest) validateID(formats strfmt.Registry) error {

	if swag.IsZero(m.ID) { // not required
		return nil
	}

	if m.ID != nil {
		if err := m.ID.Validate(formats); err != nil {
			if ve, ok := err.(*errors.Validation); ok {
				return ve.ValidateName("id")
			}
			return err
		}
	}

	return nil
}

// MarshalBinary interface implementation
func (m *APIRDSAddRequest) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *APIRDSAddRequest) UnmarshalBinary(b []byte) error {
	var res APIRDSAddRequest
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}