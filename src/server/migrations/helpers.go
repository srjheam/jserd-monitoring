package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
)

// dropCollections deletes each named collection, in the given order. Order
// matters: a collection a relation field points at must go last.
func dropCollections(app core.App, names ...string) error {
	for _, name := range names {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return err
		}
		if err := app.Delete(collection); err != nil {
			return err
		}
	}
	return nil
}

// saveCollections saves each collection, in the given order. Order matters: a
// relation field needs the collection it points at to exist already.
func saveCollections(app core.App, collections ...*core.Collection) error {
	for _, collection := range collections {
		if err := app.Save(collection); err != nil {
			return err
		}
	}
	return nil
}

// findCollections loads each named collection.
func findCollections(app core.App, names ...string) ([]*core.Collection, error) {
	found := make([]*core.Collection, 0, len(names))
	for _, name := range names {
		collection, err := app.FindCollectionByNameOrId(name)
		if err != nil {
			return nil, err
		}
		found = append(found, collection)
	}
	return found, nil
}

// selectField reads one select field of a collection.
func selectField(collection *core.Collection, name string) (*core.SelectField, error) {
	field, ok := collection.Fields.GetByName(name).(*core.SelectField)
	if !ok {
		return nil, fmt.Errorf("%s %s field is invalid", collection.Name, name)
	}
	return field, nil
}
