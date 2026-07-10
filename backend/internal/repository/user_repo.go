package repository

import (
	"encoding/json"

	"vpn-sub/internal/models"
)

// UserRepo provides access to the users table.
type UserRepo struct{}

func NewUserRepo() *UserRepo {
	return &UserRepo{}
}

// Create inserts a new user.
func (r *UserRepo) Create(user *models.User) error {
	groupsJSON, _ := json.Marshal(user.Groups)
	_, err := DB.Exec(
		`INSERT INTO users (user_id, username, email, role, is_advanced, groups) VALUES (?, ?, ?, ?, ?, ?)`,
		user.UserID, user.Username, user.Email, user.Role, boolToInt(user.IsAdvanced), string(groupsJSON),
	)
	return err
}

// FindByID retrieves a user by user_id.
func (r *UserRepo) FindByID(userID string) (*models.User, error) {
	u := &models.User{}
	var isAdvanced int
	var groupsJSON string
	err := DB.QueryRow(
		`SELECT user_id, username, email, role, is_advanced, groups FROM users WHERE user_id = ?`,
		userID,
	).Scan(&u.UserID, &u.Username, &u.Email, &u.Role, &isAdvanced, &groupsJSON)
	if err != nil {
		return nil, err
	}
	u.IsAdvanced = isAdvanced != 0
	if err := json.Unmarshal([]byte(groupsJSON), &u.Groups); err != nil {
		u.Groups = []string{}
	}
	return u, nil
}

// Update updates a user's editable fields (username, email, is_advanced, groups).
func (r *UserRepo) Update(user *models.User) error {
	groupsJSON, _ := json.Marshal(user.Groups)
	_, err := DB.Exec(
		`UPDATE users SET username = ?, email = ?, is_advanced = ?, groups = ? WHERE user_id = ?`,
		user.Username, user.Email, boolToInt(user.IsAdvanced), string(groupsJSON), user.UserID,
	)
	return err
}

// UpdateRole updates only the role field.
func (r *UserRepo) UpdateRole(userID, role string) error {
	_, err := DB.Exec(`UPDATE users SET role = ? WHERE user_id = ?`, role, userID)
	return err
}

// Delete removes a user by user_id.
func (r *UserRepo) Delete(userID string) error {
	_, err := DB.Exec(`DELETE FROM users WHERE user_id = ?`, userID)
	return err
}

// List returns all users.
func (r *UserRepo) List() ([]models.User, error) {
	rows, err := DB.Query(`SELECT user_id, username, email, role, is_advanced, groups FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		var isAdvanced int
		var groupsJSON string
		if err := rows.Scan(&u.UserID, &u.Username, &u.Email, &u.Role, &isAdvanced, &groupsJSON); err != nil {
			return nil, err
		}
		u.IsAdvanced = isAdvanced != 0
		if err := json.Unmarshal([]byte(groupsJSON), &u.Groups); err != nil {
			u.Groups = []string{}
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// CountByRole returns the number of users with a given role.
func (r *UserRepo) CountByRole(role string) (int, error) {
	var count int
	err := DB.QueryRow(`SELECT COUNT(*) FROM users WHERE role = ?`, role).Scan(&count)
	return count, err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
