package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/migrations"
)

func init() {
	migrations.Register(addEditorialDeliveryLease, dropEditorialDeliveryLease)
}

// addEditorialDeliveryLease adds the "sending" status and the lease fields, so
// exactly one sender owns one editorial summary e-mail.
func addEditorialDeliveryLease(app core.App) error {
	attempts, status, err := editorialAttemptStatus(app)
	if err != nil {
		return err
	}
	status.Values = []string{"sending", statusValueSent, statusValueBlocked, statusValueFailed}
	attempts.Fields.Add(&core.TextField{Name: "lease_owner"}, &core.DateField{Name: "lease_expires_at"})
	return app.Save(attempts)
}

func dropEditorialDeliveryLease(app core.App) error {
	attempts, status, err := editorialAttemptStatus(app)
	if err != nil {
		return err
	}
	status.Values = []string{statusValueSent, statusValueBlocked, statusValueFailed}
	attempts.Fields.RemoveByName("lease_owner")
	attempts.Fields.RemoveByName("lease_expires_at")
	return app.Save(attempts)
}

// editorialAttemptStatus loads the editorial delivery attempts collection and
// its status field.
func editorialAttemptStatus(app core.App) (*core.Collection, *core.SelectField, error) {
	attempts, err := app.FindCollectionByNameOrId("editorial_delivery_attempts")
	if err != nil {
		return nil, nil, err
	}
	status, err := selectField(attempts, fieldStatus)
	if err != nil {
		return nil, nil, err
	}
	return attempts, status, nil
}
