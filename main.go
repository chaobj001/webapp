package main

import (
	"database/sql"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL"
	"html/template"
	"log"
	"net/http"
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
	http.HandleFunc("/post", makeHandler(postHandler))
	//http.HandleFunc("/edit", makeHandler(editHandler))
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
		t, _ := template.ParseFiles("publish.html")
		t.Execute(w, nil)
	} else {
		r.ParseForm() //解析参数，默认是不会解析的
		//fmt.Println(r.Form) //这些信息是输出到服务器端的打印信息
		//fmt.Println(r.Form["title"])
		//.Println(r.FormValue("title"))
		//fmt.Println(r.Form.Get("time"))
		//fmt.Println("path", r.URL.Path) // /add
		//fmt.Println("scheme", r.URL.Scheme)

		//db
		db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/test?charset=utf8")
		checkErr(err)

		//插入數據
		stmt, err := db.Prepare("INSERT posts SET title=?, content=?, time=?")
		checkErr(err)

		res, err := stmt.Exec(r.FormValue("title"), r.FormValue("content"), time.Now().Unix())
		checkErr(err)

		id, err := res.LastInsertId()
		checkErr(err)

		fmt.Println(id)

		http.Redirect(w, r, "/posts", http.StatusFound)
	}
}

func postsHandler(w http.ResponseWriter, r *http.Request) {
	//db
	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/test?charset=utf8")
	checkErr(err)

	//查詢數據
	rows, err := db.Query("select id, title, content, time from posts order by id desc limit ?, ?", 0, 10)
	checkErr(err)
	defer rows.Close()

	type Page struct {
		Posts []*Post
	}

	var data Page

	for rows.Next() {
		var id int
		var title string
		var content string
		var time int
		if err := rows.Scan(&id, &title, &content, &time); err != nil {
			log.Fatal(err)
		}

		data.Posts = append(data.Posts, &Post{Id: id, Title: title, Content: content, Time: time})
	}
	//fmt.Println(data)
	//模板應用
	t, _ := template.ParseFiles("posts.html")
	t.Execute(w, data)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	var post_id = r.Form.Get("post_id")
	var (
		id        int
		title     string
		content   string
		timestamp int
	)

	type Page struct {
		Post *Post
	}

	var data Page

	//db
	db, err := sql.Open("mysql", "root:root@tcp(localhost:3306)/test?charset=utf8")
	checkErr(err)

	//查詢數據
	err = db.QueryRow("select id, title, content, time from posts where id=?", post_id).Scan(&id, &title, &content, &timestamp)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No user with that ID.")
		http.NotFound(w, r)
	case err != nil:
		log.Fatal(err)
	default:
		data.Post = &Post{Id: id, Title: title, Content: content, Date: time.Unix(int64(timestamp), 0).Format("2006-01-02 15:04:05")}
		//模板應用
		t, _ := template.ParseFiles("post.html")
		t.Execute(w, data)
	}

}

func checkErr(err error) {
	if err != nil {
		log.Fatal("err: ", err)
	}
}
