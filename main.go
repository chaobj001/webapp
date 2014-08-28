package main

import (
	"database/sql"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Post struct {
	Id      int
	Title   string
	Content string
	Time    int
	Date    string
}

func main() {
	//route
	fileServer := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	http.Handle("/static/", fileServer)
	fileServer = http.StripPrefix("/html/", http.FileServer(http.Dir("html")))
	http.Handle("/html/", fileServer)
	http.HandleFunc("/post", makeHandler(postHandler))
	http.HandleFunc("/edit", makeHandler(editHandler))
	http.HandleFunc("/posts", makeHandler(postsHandler))
	http.HandleFunc("/publish", makeHandler(publishHandler))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func makeHandler(fn func(http.ResponseWriter, *http.Request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fn(w, r)
	}
}

func publishHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//模板應用
		t, _ := template.ParseFiles("html/publish.html")
		t.Execute(w, nil)
	} else {
		r.ParseForm() //解析参数，默认是不会解析的
		//fmt.Println(r.Form) //这些信息是输出到服务器端的打印信息
		//fmt.Println(r.Form["title"])
		//.Println(r.FormValue("title"))
		//fmt.Println(r.Form.Get("time"))
		//fmt.Println("path", r.URL.Path) // /add
		//fmt.Println("scheme", r.URL.Scheme)

		title := strings.TrimSpace(r.FormValue("title"))
		content := r.FormValue("content")
		create_time := time.Now().Unix()

		if len(title) == 0 {
			http.Error(w, "標題不能為空", http.StatusForbidden)
			return
		}
		if len(content) == 0 {
			http.Error(w, "內容不能為空", http.StatusForbidden)
			return
		}

		//db
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/webapp?charset=utf8")
		checkErr(err)
		//插入數據
		stmt, err := db.Prepare("INSERT posts SET title=?, content=?, create_time=?")
		checkErr(err)
		res, err := stmt.Exec(title, content, create_time)
		checkErr(err)
		id, err := res.LastInsertId()
		checkErr(err)
		fmt.Printf("Insert Id: %d\n", id)
		http.Redirect(w, r, "/posts", http.StatusFound)
	}
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	limit := 5
	p, _ := strconv.Atoi(r.Form.Get("page"))
	if p == 0 {
		p = 1
	}
	start := limit * (p - 1)

	//db
	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢數據
	rows, err := db.Query("select id, title, content, create_time from posts order by id desc limit ?, ?", start, limit)
	checkErr(err)
	defer rows.Close()
	type Page struct {
		Posts   []*Post
		PageNav interface{}
	}
	var data Page
	for rows.Next() {
		var id int
		var title string
		var content string
		var create_time int
		if err := rows.Scan(&id, &title, &content, &create_time); err != nil {
			log.Fatal(err)
		}
		data.Posts = append(data.Posts, &Post{Id: id, Title: title, Content: content, Time: create_time})
	}

	//獲取總條數
	var total int
	err = db.QueryRow("select count(id) as total from posts").Scan(&total)
	if err == sql.ErrNoRows || err != nil {
		total = 0
	}

	//分頁
	page := &PageNav{url: "/posts?page=", pagesize: limit, length: total}
	data.PageNav = page.getPage()

	//fmt.Println(data)
	//模板應用
	t, _ := template.ParseFiles("html/posts.html")
	t.Execute(w, data)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	post_id, _ := strconv.Atoi(r.Form.Get("post_id"))
	if post_id == 0 {
		http.NotFound(w, r)
		return
	}

	var (
		id          int
		title       string
		content     string
		create_time int
	)
	type Page struct {
		Post *Post
	}
	var data Page
	//db
	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢數據
	err = db.QueryRow("select id, title, content, create_time from posts where id=?", post_id).Scan(&id, &title, &content, &create_time)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No user with that ID.")
		http.NotFound(w, r)
	case err != nil:
		log.Fatal(err)
	default:
		data.Post = &Post{Id: id, Title: title, Content: content, Date: time.Unix(int64(create_time), 0).Format("2006-01-02 15:04:05")}
		//模板應用
		t, _ := template.ParseFiles("html/post.html")
		t.Execute(w, data)
	}

}

func editHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	if r.Method == "GET" {
		post_id, _ := strconv.Atoi(r.Form.Get("post_id"))

		if post_id == 0 {
			http.NotFound(w, r)
			return
		}

		var (
			id      int
			title   string
			content string
		)
		type Data struct {
			Id      int
			Title   string
			Content string
		}
		//db
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/webapp?charset=utf8")
		//查詢數據
		err = db.QueryRow("select id, title, content from posts where id=?", post_id).Scan(&id, &title, &content)
		switch {
		case err == sql.ErrNoRows:
			log.Printf("No user with that ID.")
			http.NotFound(w, r)
		case err != nil:
			log.Fatal(err)
		default:
			data := &Data{Id: id, Title: title, Content: content}
			//模板應用
			t, _ := template.ParseFiles("html/edit.html")
			t.Execute(w, data)
		}
	} else {
		id, _ := strconv.Atoi(r.FormValue("id"))
		title := strings.TrimSpace(r.FormValue("title"))
		content := r.FormValue("content")
		update_time := time.Now().Unix()

		if id == 0 {
			http.NotFound(w, r)
			return
		}

		if len(title) == 0 {
			http.Error(w, "標題不能為空", http.StatusForbidden)
			return
		}
		if len(content) == 0 {
			http.Error(w, "內容不能為空", http.StatusForbidden)
			return
		}

		//db
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/webapp?charset=utf8")
		checkErr(err)
		//插入數據
		stmt, err := db.Prepare("update posts SET title=?, content=?, update_time=? where id=?")
		checkErr(err)
		res, err := stmt.Exec(title, content, update_time, id)
		checkErr(err)
		affect, err := res.RowsAffected()
		checkErr(err)
		fmt.Println(affect)
		http.Redirect(w, r, "/posts", http.StatusFound)
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal("err: ", err)
	}
}

type PageNav struct {
	url      string
	pagesize int
	length   int
}

func (p *PageNav) getPage() interface{} {
	d := float64(p.length) / float64(p.pagesize)
	pages := int(math.Ceil(d))
	if pages == 1 {
		return ""
	}
	html := ""
	for i := 1; i <= pages; i++ {
		//fmt.Println(i)
		j := strconv.Itoa(i)
		html += "<a href=\"" + p.url + j + "\">" + j + "</a>"
	}
	return template.HTML(html)
}
