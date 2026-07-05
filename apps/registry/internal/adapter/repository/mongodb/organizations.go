package mongodb

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Organization is the mongo-persisted org document.
type Organization struct {
	ID          string      `bson:"_id"`
	Slug        string      `bson:"slug"`
	Name        string      `bson:"name"`
	Description string      `bson:"description,omitempty"`
	Visibility  string      `bson:"default_visibility,omitempty"`
	CreatedBy   string      `bson:"created_by"`
	CreatedAt   string      `bson:"created_at"`
	UpdatedAt   string      `bson:"updated_at"`
	Members     []OrgMember `bson:"members"`
}

type OrgMember struct {
	UserID string `bson:"user_id"`
	Email  string `bson:"email,omitempty"`
	Role   string `bson:"role"`
}

var ErrOrgNotFound = errors.New("organization not found")

func (r *Repository) CreateOrganization(ctx context.Context, org Organization) (*Organization, error) {
	if !r.Enabled() {
		return nil, fmt.Errorf("mongodb repository is not configured")
	}
	if _, err := r.organizations.InsertOne(ctx, org); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, fmt.Errorf("organization already exists")
		}
		return nil, err
	}
	return &org, nil
}

func (r *Repository) ListOrganizations(ctx context.Context, memberUserID string) ([]*Organization, error) {
	if !r.Enabled() {
		return nil, fmt.Errorf("mongodb repository is not configured")
	}
	filter := bson.M{}
	if strings.TrimSpace(memberUserID) != "" {
		filter["members.user_id"] = strings.TrimSpace(memberUserID)
	}
	cur, err := r.organizations.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "slug", Value: 1}}))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := []*Organization{}
	for cur.Next(ctx) {
		var org Organization
		if err := cur.Decode(&org); err != nil {
			return nil, err
		}
		cp := org
		out = append(out, &cp)
	}
	return out, cur.Err()
}

func (r *Repository) GetOrganization(ctx context.Context, slug string) (*Organization, error) {
	if !r.Enabled() {
		return nil, fmt.Errorf("mongodb repository is not configured")
	}
	var org Organization
	err := r.organizations.FindOne(ctx, bson.M{"slug": strings.TrimSpace(slug)}).Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// SetMembers replaces the org's member list and bumps updated_at.
func (r *Repository) SetMembers(ctx context.Context, slug string, members []OrgMember, updatedAt string) (*Organization, error) {
	if !r.Enabled() {
		return nil, fmt.Errorf("mongodb repository is not configured")
	}
	res := r.organizations.FindOneAndUpdate(
		ctx,
		bson.M{"slug": strings.TrimSpace(slug)},
		bson.M{"$set": bson.M{"members": members, "updated_at": updatedAt}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	)
	var org Organization
	err := res.Decode(&org)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrOrgNotFound
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}
