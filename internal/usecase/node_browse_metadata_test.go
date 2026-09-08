package usecase

import (
	"context"
	"errors"
	"omniflow-go/internal/actor"
	domainnode "omniflow-go/internal/domain/node"
	"testing"
)

type metadataReaderFixture struct {
	rootReads, queryReads int
	badParent             bool
	after                 uint64
	query                 domainnode.MetadataQuery
}

func (f *metadataReaderFixture) ReadMetadataRoot(context.Context, uint64) (domainnode.MetadataEntry, error) {
	f.rootReads++
	return domainnode.MetadataEntry{ID: 10, LibraryID: 3, Name: "root", Type: domainnode.TypeDirectory, Path: "/"}, nil
}
func (f *metadataReaderFixture) ReadMetadataNode(_ context.Context, id, libraryID uint64) (domainnode.MetadataEntry, error) {
	if f.badParent {
		libraryID++
	}
	return domainnode.MetadataEntry{ID: id, LibraryID: libraryID, Type: domainnode.TypeDirectory}, nil
}
func (f *metadataReaderFixture) QueryMetadata(_ context.Context, q domainnode.MetadataQuery, _ uint64, after uint64, limit int) ([]domainnode.MetadataEntry, error) {
	f.queryReads++
	f.after = after
	f.query = q
	rows := []domainnode.MetadataEntry{}
	for _, id := range []uint64{20, 21, 22} {
		if id > after && len(rows) < limit {
			rows = append(rows, domainnode.MetadataEntry{ID: id, LibraryID: 3, ParentID: 10, Name: "file", Type: domainnode.TypeFile})
		}
	}
	return rows, nil
}
func metadataReadAllowed(context.Context, actor.Actor, uint64) error { return nil }

func TestBrowseMetadataPaginationAndCursorScope(t *testing.T) {
	r := &metadataReaderFixture{}
	input := BrowseNodeMetadataQuery{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "children"}, Limit: 2}
	first, err := browseNodeMetadata(context.Background(), input, r, metadataReadAllowed)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Entries) != 2 || !first.HasMore || first.NextCursor == "" || r.query.ParentID != 10 {
		t.Fatalf("unexpected first page: %+v", first)
	}
	input.Cursor = first.NextCursor
	last, err := browseNodeMetadata(context.Background(), input, r, metadataReadAllowed)
	if err != nil || len(last.Entries) != 1 || last.Entries[0].ID != 22 || last.HasMore || r.after != 21 {
		t.Fatalf("unexpected continuation: %+v %v", last, err)
	}
	input.Query.Keyword = "changed"
	if _, err = browseNodeMetadata(context.Background(), input, r, metadataReadAllowed); !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("cursor scope not checked: %v", err)
	}
	if r.queryReads != 2 {
		t.Fatal("invalid cursor reached repository query")
	}
}

func TestBrowseMetadataAuthorizationAndParentScope(t *testing.T) {
	r := &metadataReaderFixture{}
	denied := errors.New("denied")
	input := BrowseNodeMetadataQuery{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "children", ParentID: 40}}
	_, err := browseNodeMetadata(context.Background(), input, r, func(context.Context, actor.Actor, uint64) error { return denied })
	if !errors.Is(err, denied) || r.rootReads != 0 {
		t.Fatal("authorization must precede reads")
	}
	r.badParent = true
	_, err = browseNodeMetadata(context.Background(), input, r, metadataReadAllowed)
	if !errors.Is(err, ErrNotFound) || r.queryReads != 0 {
		t.Fatalf("foreign parent accepted: %v", err)
	}
}

func TestBrowseMetadataRootIsReadOnlyAndBounded(t *testing.T) {
	r := &metadataReaderFixture{}
	page, err := browseNodeMetadata(context.Background(), BrowseNodeMetadataQuery{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "root"}}, r, metadataReadAllowed)
	if err != nil || page.Root.ID != 10 || len(page.Entries) != 0 || page.Entries == nil || r.queryReads != 0 {
		t.Fatalf("unexpected root read: %+v %v", page, err)
	}
	for _, input := range []BrowseNodeMetadataQuery{
		{Query: domainnode.MetadataQuery{LibraryID: 3}, Limit: 101},
		{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "search", ParentID: 10}},
		{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "node"}},
		{Query: domainnode.MetadataQuery{LibraryID: 3, Mode: "node", NodeID: 20}, Cursor: "invalid"},
	} {
		if _, err := browseNodeMetadata(context.Background(), input, r, metadataReadAllowed); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("invalid input accepted: %+v %v", input, err)
		}
	}
}
