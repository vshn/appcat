// Code generated by angryjet. DO NOT EDIT.

package v1

import resource "github.com/crossplane/crossplane-runtime/pkg/resource"

// GetItems of this XVSHNMariaDBList.
func (l *XVSHNMariaDBList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}

// GetItems of this XVSHNMinioList.
func (l *XVSHNMinioList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}

// GetItems of this XVSHNPostgreSQLList.
func (l *XVSHNPostgreSQLList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}

// GetItems of this XVSHNRedisList.
func (l *XVSHNRedisList) GetItems() []resource.Managed {
	items := make([]resource.Managed, len(l.Items))
	for i := range l.Items {
		items[i] = &l.Items[i]
	}
	return items
}