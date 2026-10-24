package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

type Item struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Rating   float32 `json:"rating"`
	Image    string  `json:"image"`
	Category string  `json:"category"`
}

// listItems serves both /items and /category/{category}.
func (a *App) listItems(w http.ResponseWriter, r *http.Request) {
	cid := a.canteenID(w, r)
	if cid == 0 {
		return
	}
	q, args := "SELECT id,name,price,rating,image,category FROM items WHERE canteen_id=?", []any{cid}
	if c := r.PathValue("category"); c != "" {
		q += " AND category=?"
		args = append(args, c)
	}
	rows, err := a.db.Query(q, args...)
	if err != nil {
		fail(w, 500, "db error")
		return
	}
	defer rows.Close()
	items := []Item{}
	for rows.Next() {
		var it Item
		if err := rows.Scan(&it.ID, &it.Name, &it.Price, &it.Rating, &it.Image, &it.Category); err != nil {
			fail(w, 500, "db error")
			return
		}
		items = append(items, it)
	}
	writeJSON(w, 200, items)
}

// flexFloat accepts 12.5 or "12.5": the admin form posts every field as a string.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) != nil {
		s = string(b)
	}
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	*f = flexFloat(v)
	return err
}

func (a *App) addItem(w http.ResponseWriter, r *http.Request, cid int64) {
	var in struct {
		Name     string    `json:"name"`
		Price    flexFloat `json:"price"`
		Rating   flexFloat `json:"rating"`
		Image    string    `json:"image"`
		Category string    `json:"category"`
	}
	if !decode(w, r, &in) {
		return
	}
	it := Item{Name: in.Name, Price: float64(in.Price), Rating: float32(in.Rating), Image: in.Image, Category: in.Category}
	if it.Name == "" || it.Price <= 0 {
		fail(w, 400, "name and positive price required")
		return
	}
	res, err := a.db.Exec("INSERT INTO items (canteen_id,name,price,rating,image,category) VALUES (?,?,?,?,?,?)",
		cid, it.Name, it.Price, it.Rating, it.Image, it.Category)
	if err != nil {
		fail(w, 500, "db error")
		return
	}
	it.ID, _ = res.LastInsertId()
	writeJSON(w, 201, it)
}
