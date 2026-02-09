package ui

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/myuon/track/internal/store/sqlite"
)

const listTpl = `<!doctype html><html><body><h1>Track Issues</h1><ul>{{range .}}<li><a href="/issues/{{.ID}}">{{.ID}}</a> [{{.Status}}] {{.Title}}</li>{{else}}<li>No issues</li>{{end}}</ul></body></html>`
const detailTpl = `<!doctype html><html><body><p><a href="/">Back</a></p><h1>{{.ID}} {{.Title}}</h1><p>Status: {{.Status}}</p><p>Priority: {{.Priority}}</p><form method="post" action="/issues/{{.ID}}/edit"><label>Title <input name="title" value="{{.Title}}"></label><br><label>Body <textarea name="body">{{.Body}}</textarea></label><br><button type="submit">Save</button></form></body></html>`

func NewHandler() http.Handler {
	listT := template.Must(template.New("list").Parse(listTpl))
	detailT := template.Must(template.New("detail").Parse(detailTpl))

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		ctx := context.Background()
		store, err := sqlite.Open(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer store.Close()

		items, err := store.ListIssues(ctx, sqlite.ListFilter{Sort: "manual"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := listT.Execute(w, items); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	mux.HandleFunc("/issues/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/issues/")
		if rest == "" {
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(rest, "/edit") {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			id := strings.TrimSuffix(rest, "/edit")
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			title := r.Form.Get("title")
			body := r.Form.Get("body")
			in := sqlite.UpdateIssueInput{Title: &title, Body: &body}

			ctx := context.Background()
			store, err := sqlite.Open(ctx)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer store.Close()
			if _, err := store.UpdateIssue(ctx, id, in); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/issues/"+id, http.StatusSeeOther)
			return
		}

		ctx := context.Background()
		store, err := sqlite.Open(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer store.Close()

		it, err := store.GetIssue(ctx, rest)
		if err != nil {
			http.Error(w, fmt.Sprintf("issue not found: %v", err), http.StatusNotFound)
			return
		}
		if err := detailT.Execute(w, it); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	return mux
}
