package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const tagsCollection = "tags"

type Tag struct {
	Slug      string    `firestore:"-"`
	Name      string    `firestore:"name"`
	CreatedAt time.Time `firestore:"createdAt,serverTimestamp"`
}

type TagStore struct {
	client *firestore.Client
}

func NewTagStore(client *firestore.Client) *TagStore {
	return &TagStore{client: client}
}

func (t *TagStore) Get(ctx context.Context, slug string) (*Tag, error) {
	if !ValidSlug(slug) {
		return nil, ErrNotFound
	}

	snap, err := t.client.Collection(tagsCollection).Doc(slug).Get(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, ErrNotFound
		}
		return nil, err
	}

	var tag Tag
	if err := snap.DataTo(&tag); err != nil {
		return nil, err
	}
	tag.Slug = snap.Ref.ID

	return &tag, nil
}

func (t *TagStore) List(ctx context.Context) ([]Tag, error) {
	snaps, err := t.client.Collection(tagsCollection).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	var tags []Tag

	for _, snap := range snaps {
		var tag Tag
		if err := snap.DataTo(&tag); err != nil {
			return nil, err
		}
		tag.Slug = snap.Ref.ID
		tags = append(tags, tag)
	}

	return tags, nil
}

func (t *TagStore) Create(ctx context.Context, slug, name string) (*Tag, error) {
	ref := t.client.Collection(tagsCollection).Doc(slug)

	var tag Tag
	tag.Name = name

	_, err := ref.Create(ctx, tag)
	if err != nil {
		if status.Code(err) == codes.AlreadyExists {
			return nil, ErrSlugConflict
		}
		return nil, err
	}
	tag.Slug = ref.ID
	return &tag, nil
}

func (t *TagStore) Rename(ctx context.Context, slug, name string) (*Tag, error) {
	tag, err := t.Get(ctx, slug)
	if err != nil {
		return nil, err
	}

	var ups []firestore.Update
	if tag.Name != name {
		ups = append(ups, firestore.Update{Path: "name", Value: name})
	} else {
		return tag, nil
	}

	doc := t.client.Collection(tagsCollection).Doc(slug)

	_, err = doc.Update(ctx, ups)
	if err != nil {
		return nil, err
	}

	return t.Get(ctx, slug)
}
