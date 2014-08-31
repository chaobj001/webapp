package main

import (
	"database/sql"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL"
	"github.com/russross/blackfriday"
	"html/template"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Post struct {
	Id        int
	Parent_id int
	Uid       int
	Title     string
	Content   interface{}
	Time      int
	Date      string
}

func main() {
	//目录设置
	fileServer := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	http.Handle("/static/", fileServer)
	fileServer = http.StripPrefix("/html/", http.FileServer(http.Dir("html")))
	http.Handle("/html/", fileServer)
	//路由设置
	http.HandleFunc("/post", makeHandler(postHandler))
	http.HandleFunc("/edit", makeHandler(editHandler))
	http.HandleFunc("/posts", makeHandler(postsHandler))
	http.HandleFunc("/publish", makeHandler(publishHandler))
	http.HandleFunc("/reply", makeHandler(replyHandler))
	http.HandleFunc("/reg", makeHandler(regHandler))
	http.HandleFunc("/login", makeHandler(loginHandler))
	http.HandleFunc("/logout", makeHandler(logoutHandler))
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

//发布
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
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(127.0.0.1:3306)/webapp?charset=utf8")
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

//列表页
func postsHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	limit := 5
	p, _ := strconv.Atoi(r.Form.Get("page"))
	if p == 0 {
		p = 1
	}
	start := limit * (p - 1)

	//db
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(127.0.0.1:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢數據
	rows, err := db.Query("select id, title, content, create_time from posts where parent_id=0 order by id desc limit ?, ?", start, limit)
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
	err = db.QueryRow("select count(id) as total from posts where parent_id = 0").Scan(&total)
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

//post详情
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
		Post    *Post
		Replies []*Post
	}
	var data Page
	//db
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(127.0.0.1:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢POST數據
	err = db.QueryRow("select id, title, content, create_time from posts where id=?", post_id).Scan(&id, &title, &content, &create_time)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No user with that ID.")
		http.NotFound(w, r)
	case err != nil:
		log.Fatal(err)
	default:
		data.Post = &Post{Id: id, Title: title, Content: template.HTML(string(blackfriday.MarkdownCommon([]byte(content)))), Date: time.Unix(int64(create_time), 0).Format("2006-01-02 15:04:05")}
	}
	//查詢reply數據
	replies, err := db.Query("select id, content, create_time from posts where parent_id=? order by id desc limit ?, ?", post_id, 0, 20)
	checkErr(err)
	defer replies.Close()
	for replies.Next() {
		var id int
		var content string
		var create_time int
		if err := replies.Scan(&id, &content, &create_time); err != nil {
			log.Fatal(err)
		}
		data.Replies = append(data.Replies, &Post{Id: id, Content: content, Date: time.Unix(int64(create_time), 0).Format("2006-01-02 15:04:05")})
	}
	//模板應用
	t, _ := template.ParseFiles("html/post.html")
	t.Execute(w, data)
}

//编辑
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
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(127.0.0.1:3306)/webapp?charset=utf8")
		defer db.Close()
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
		defer db.Close()
		//插入數據
		stmt, err := db.Prepare("update posts SET title=?, content=?, update_time=? where id=?")
		checkErr(err)
		res, err := stmt.Exec(title, content, update_time, id)
		checkErr(err)
		affect, err := res.RowsAffected()
		checkErr(err)
		if affect > 0 {
			http.Redirect(w, r, "/post?post_id="+strconv.Itoa(id), http.StatusFound)
		} else {
			http.Error(w, "affect", http.StatusForbidden)
		}

	}
}

//回复
func replyHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	//fmt.Println(r.Form)
	//fmt.Println(r.PostForm)
	reply_id, _ := strconv.Atoi(r.FormValue("id"))
	reply_content := strings.TrimSpace(r.FormValue("content"))
	create_time := time.Now().Unix()
	if reply_id == 0 {
		http.Error(w, "回复id不存在", http.StatusForbidden)
		return
	}
	if len(reply_content) == 0 {
		http.Error(w, "回复内存为空", http.StatusForbidden)
		return
	}
	//db
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(127.0.0.1:3306)/webapp?charset=utf8")
	checkErr(err)
	defer db.Close()
	//插入數據
	insetReply, err := db.Prepare("INSERT posts SET parent_id=?, content=?, create_time=?")
	checkErr(err)
	//更新post replies
	updatePost, err := db.Prepare("update posts set replies = replies + 1, last_reply_time = ? where id = ?")
	checkErr(err)
	//开启事务
	tx, err := db.Begin()
	_, err = tx.Stmt(insetReply).Exec(reply_id, reply_content, create_time)
	if err != nil {
		fmt.Println("err sql insert reply")
	} else {
		_, err = tx.Stmt(updatePost).Exec(create_time, reply_id)
		if err != nil {
			fmt.Println("err sql update post by reply")
		} else {
			tx.Commit()
		}
	}
	if err != nil {
		tx.Rollback()
		http.Error(w, "事务失败", http.StatusForbidden)
	}
	http.Redirect(w, r, "/post?post_id="+strconv.Itoa(reply_id), http.StatusFound)
}

func regHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//模板應用
		t, _ := template.ParseFiles("html/reg.html")
		t.Execute(w, nil)
	} else {

	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {

}

func logoutHandler(w http.ResponseWriter, r *http.Request) {

}

func checkErr(err error) {
	if err != nil {
		log.Fatal("err: ", err)
	}
}

//分页
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

// func unescaped (x string) interface{} { return template.HTML(x) }
// template.FuncMap{"unescaped": unescaped}
