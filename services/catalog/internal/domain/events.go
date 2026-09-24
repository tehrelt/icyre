package domain

// Event is a fact about the catalog that other services may react to.
// Its wire format is defined by the Kafka adapter, not here.
type Event interface {
	isCatalogEvent()
}

// ArtistCreated is raised after an artist is stored.
type ArtistCreated struct{ Artist Artist }

// AlbumCreated is raised after an album is stored.
type AlbumCreated struct{ Album Album }

// TrackCreated is raised after a track is stored.
type TrackCreated struct{ Track Track }

// TrackUpdated is raised after a stored track changes.
type TrackUpdated struct{ Track Track }

func (ArtistCreated) isCatalogEvent() {}
func (AlbumCreated) isCatalogEvent()  {}
func (TrackCreated) isCatalogEvent()  {}
func (TrackUpdated) isCatalogEvent()  {}
