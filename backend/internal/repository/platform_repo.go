package repository

import (
	"encoding/json"

	"vpn-sub/internal/models"
)

// PlatformRepo provides access to the platforms table.
type PlatformRepo struct{}

func NewPlatformRepo() *PlatformRepo {
	return &PlatformRepo{}
}

func (r *PlatformRepo) FindByID(id string) (*models.Platform, error) {
	p := &models.Platform{}
	var schemesJSON string
	err := DB.QueryRow(
		`SELECT id, name, description, client_schemes, download_url FROM platforms WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &schemesJSON, &p.DownloadURL)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(schemesJSON), &p.ClientSchemes); err != nil {
		p.ClientSchemes = []string{}
	}
	return p, nil
}

func (r *PlatformRepo) List() ([]models.Platform, error) {
	rows, err := DB.Query(`SELECT id, name, description, client_schemes, download_url FROM platforms ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var platforms []models.Platform
	for rows.Next() {
		var p models.Platform
		var schemesJSON string
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &schemesJSON, &p.DownloadURL); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(schemesJSON), &p.ClientSchemes); err != nil {
			p.ClientSchemes = []string{}
		}
		platforms = append(platforms, p)
	}
	return platforms, rows.Err()
}

func (r *PlatformRepo) Create(p *models.Platform) error {
	schemesJSON, _ := json.Marshal(p.ClientSchemes)
	_, err := DB.Exec(
		`INSERT INTO platforms (id, name, description, client_schemes, download_url) VALUES (?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Description, string(schemesJSON), p.DownloadURL,
	)
	return err
}

func (r *PlatformRepo) Update(p *models.Platform) error {
	schemesJSON, _ := json.Marshal(p.ClientSchemes)
	_, err := DB.Exec(
		`UPDATE platforms SET name = ?, description = ?, client_schemes = ?, download_url = ? WHERE id = ?`,
		p.Name, p.Description, string(schemesJSON), p.DownloadURL, p.ID,
	)
	return err
}

func (r *PlatformRepo) Delete(id string) error {
	_, err := DB.Exec(`DELETE FROM platforms WHERE id = ?`, id)
	return err
}
