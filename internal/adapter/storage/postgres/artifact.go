package postgres

import (
	"context"

	"github.com/google/uuid"

	"vk-ai-aggregator/internal/domain"
)

// ArtifactRepository is the PostgreSQL implementation of
// domain.ArtifactRepository.
type ArtifactRepository struct {
	db Querier
}

// NewArtifactRepository builds an ArtifactRepository over the given querier.
func NewArtifactRepository(db Querier) *ArtifactRepository {
	return &ArtifactRepository{db: db}
}

var _ domain.ArtifactRepository = (*ArtifactRepository)(nil)

const artifactColumns = `id, owner_user_id, job_id, kind, media_type, mime_type,
	storage_bucket, storage_key, public_url, sha256, size_bytes, width, height,
	duration_ms, codec, container, bitrate_bps, probe_status, status, created_at, updated_at`

const artifactVariantColumns = `id, artifact_id, variant_type, storage_bucket, storage_key,
	mime_type, size_bytes, width, height, duration_ms, codec, container, bitrate_bps,
	probe_status, created_at, updated_at`

// Create inserts a new artifact.
func (r *ArtifactRepository) Create(ctx context.Context, a *domain.Artifact) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	normalizeArtifactMetadata(a)
	const q = `
		INSERT INTO artifacts (
			id, owner_user_id, job_id, kind, media_type, mime_type,
			storage_bucket, storage_key, public_url, sha256, size_bytes, width, height,
			duration_ms, codec, container, bitrate_bps, probe_status, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		RETURNING ` + artifactColumns
	row := r.db.QueryRow(ctx, q,
		a.ID, a.OwnerUserID, a.JobID, a.Kind, a.MediaType, a.MimeType,
		a.StorageBucket, a.StorageKey, a.PublicURL, a.SHA256, a.SizeBytes, a.Width, a.Height,
		a.DurationMS, a.Codec, a.Container, a.BitrateBPS, a.ProbeStatus, a.Status,
	)
	return mapError(scanArtifact(row, a))
}

// Update persists changes to an artifact.
func (r *ArtifactRepository) Update(ctx context.Context, a *domain.Artifact) error {
	normalizeArtifactMetadata(a)
	const q = `
		UPDATE artifacts
		SET kind = $2, media_type = $3, mime_type = $4, storage_bucket = $5, storage_key = $6,
		    public_url = $7, sha256 = $8, size_bytes = $9, width = $10, height = $11,
		    duration_ms = $12, codec = $13, container = $14, bitrate_bps = $15,
		    probe_status = $16, status = $17, updated_at = now()
		WHERE id = $1
		RETURNING ` + artifactColumns
	row := r.db.QueryRow(ctx, q,
		a.ID, a.Kind, a.MediaType, a.MimeType, a.StorageBucket, a.StorageKey,
		a.PublicURL, a.SHA256, a.SizeBytes, a.Width, a.Height, a.DurationMS,
		a.Codec, a.Container, a.BitrateBPS, a.ProbeStatus, a.Status,
	)
	return mapError(scanArtifact(row, a))
}

// GetByID fetches an artifact by id.
func (r *ArtifactRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Artifact, error) {
	const q = `SELECT ` + artifactColumns + ` FROM artifacts WHERE id = $1`
	var a domain.Artifact
	if err := mapError(scanArtifact(r.db.QueryRow(ctx, q, id), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// GetBySHA256 fetches an artifact by content hash for deduplication.
func (r *ArtifactRepository) GetBySHA256(ctx context.Context, ownerID uuid.UUID, sha256 string) (*domain.Artifact, error) {
	const q = `SELECT ` + artifactColumns + `
		FROM artifacts WHERE owner_user_id = $1 AND sha256 = $2
		ORDER BY created_at ASC LIMIT 1`
	var a domain.Artifact
	if err := mapError(scanArtifact(r.db.QueryRow(ctx, q, ownerID, sha256), &a)); err != nil {
		return nil, err
	}
	return &a, nil
}

// AddVariant inserts a derived variant of an artifact.
func (r *ArtifactRepository) AddVariant(ctx context.Context, v *domain.ArtifactVariant) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	normalizeArtifactVariantMetadata(v)
	const q = `
		INSERT INTO artifact_variants (
			id, artifact_id, variant_type, storage_bucket, storage_key,
			mime_type, size_bytes, width, height, duration_ms, codec, container,
			bitrate_bps, probe_status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING ` + artifactVariantColumns
	row := r.db.QueryRow(ctx, q,
		v.ID, v.ArtifactID, v.VariantType, v.StorageBucket, v.StorageKey,
		v.MimeType, v.SizeBytes, v.Width, v.Height, v.DurationMS,
		v.Codec, v.Container, v.BitrateBPS, v.ProbeStatus,
	)
	return mapError(scanArtifactVariant(row, v))
}

// ListVariants returns all variants of an artifact.
func (r *ArtifactRepository) ListVariants(ctx context.Context, artifactID uuid.UUID) ([]*domain.ArtifactVariant, error) {
	const q = `SELECT ` + artifactVariantColumns + `
		FROM artifact_variants WHERE artifact_id = $1
		ORDER BY created_at ASC`
	rows, err := r.db.Query(ctx, q, artifactID)
	if err != nil {
		return nil, mapError(err)
	}
	defer rows.Close()

	var variants []*domain.ArtifactVariant
	for rows.Next() {
		var v domain.ArtifactVariant
		if err := scanArtifactVariant(rows, &v); err != nil {
			return nil, mapError(err)
		}
		variants = append(variants, &v)
	}
	return variants, mapError(rows.Err())
}

func scanArtifact(row rowScanner, a *domain.Artifact) error {
	return row.Scan(
		&a.ID, &a.OwnerUserID, &a.JobID, &a.Kind, &a.MediaType, &a.MimeType,
		&a.StorageBucket, &a.StorageKey, &a.PublicURL, &a.SHA256, &a.SizeBytes, &a.Width, &a.Height,
		&a.DurationMS, &a.Codec, &a.Container, &a.BitrateBPS, &a.ProbeStatus, &a.Status,
		&a.CreatedAt, &a.UpdatedAt,
	)
}

func scanArtifactVariant(row rowScanner, v *domain.ArtifactVariant) error {
	return row.Scan(
		&v.ID, &v.ArtifactID, &v.VariantType, &v.StorageBucket, &v.StorageKey,
		&v.MimeType, &v.SizeBytes, &v.Width, &v.Height, &v.DurationMS,
		&v.Codec, &v.Container, &v.BitrateBPS, &v.ProbeStatus, &v.CreatedAt, &v.UpdatedAt,
	)
}

func normalizeArtifactMetadata(a *domain.Artifact) {
	m := domain.ArtifactMediaMetadata{
		Width:       a.Width,
		Height:      a.Height,
		DurationMS:  a.DurationMS,
		Codec:       a.Codec,
		Container:   a.Container,
		BitrateBPS:  a.BitrateBPS,
		ProbeStatus: a.ProbeStatus,
	}.Normalize()
	a.Width = m.Width
	a.Height = m.Height
	a.DurationMS = m.DurationMS
	a.Codec = m.Codec
	a.Container = m.Container
	a.BitrateBPS = m.BitrateBPS
	a.ProbeStatus = m.ProbeStatus
}

func normalizeArtifactVariantMetadata(v *domain.ArtifactVariant) {
	m := domain.ArtifactMediaMetadata{
		Width:       v.Width,
		Height:      v.Height,
		DurationMS:  v.DurationMS,
		Codec:       v.Codec,
		Container:   v.Container,
		BitrateBPS:  v.BitrateBPS,
		ProbeStatus: v.ProbeStatus,
	}.Normalize()
	v.Width = m.Width
	v.Height = m.Height
	v.DurationMS = m.DurationMS
	v.Codec = m.Codec
	v.Container = m.Container
	v.BitrateBPS = m.BitrateBPS
	v.ProbeStatus = m.ProbeStatus
}
