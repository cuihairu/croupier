package extensiongorm

import "gorm.io/gorm"

type Bundle struct {
	Catalog      *CatalogRepo
	Release      *ReleaseRepo
	Installation *InstallationRepo
	Binding      *BindingRepo
	Event        *EventRepo
}

func NewBundle(db *gorm.DB) *Bundle {
	return &Bundle{
		Catalog:      NewCatalogRepo(db),
		Release:      NewReleaseRepo(db),
		Installation: NewInstallationRepo(db),
		Binding:      NewBindingRepo(db),
		Event:        NewEventRepo(db),
	}
}
