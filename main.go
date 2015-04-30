package main

import (
	"crypto/md5"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/Go-SQL-Driver/MySQL"
	"github.com/russross/blackfriday"
	"html/template"
	"log"
	"math"
	"net/http"
	//"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"webapp/session"
	_ "webapp/session/memory"
)

var globalSessions *session.Manager

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
	//配置sessions
	globalSessions, _ = session.NewManager("memory", "gosessionid", 7200)
	go globalSessions.GC()

	//目录设置
	//fileServer := http.StripPrefix("/static", http.FileServer(http.Dir("/Users/wangchao/go/src/webapp/static")))
	//http.Handle("/static/", fileServer)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// 静态文件 os 绝对路径
	// wd, _ := os.Getwd() // 当前路径
	// fmt.Println(wd)

	//http.Handle("/html/", http.StripPrefix("/html", http.FileServer(http.Dir("."))))
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
		// c, _ := r.Cookie("uid")
		// name, _ := r.Cookie("username")
		// if c != nil {
		// 	fmt.Println("login uid", c, name)
		// }

		// sess := globalSessions.SessionStart(w, r)
		// fmt.Println("sess uid", sess.Get("uid"))

		fn(w, r)
	}
}

//发布
func publishHandler(w http.ResponseWriter, r *http.Request) {
	sess := globalSessions.SessionStart(w, r)
	login_uid, _ := strconv.Atoi(fmt.Sprintf("%s", sess.Get("uid")))

	if login_uid == 0 {
		http.Redirect(w, r, "/login", 302)
		return
	}

	if r.Method == "GET" {
		//模板應用
		t, _ := template.ParseFiles("html/publish.html")
		t.Execute(w, nil)
	} else {
		//解析参数，默认是不会解析的
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid request form data", 400)
			return
		}
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
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
		checkErr(err)
		//插入數據
		stmt, err := db.Prepare("INSERT posts SET uid=?, title=?, content=?, create_time=?, last_reply_time=?")
		checkErr(err)
		res, err := stmt.Exec(login_uid, title, content, create_time, create_time)
		checkErr(err)
		id, err := res.LastInsertId()
		checkErr(err)
		if id > 0 {
			http.Redirect(w, r, "/posts", 302)
		} else {
			http.Error(w, "create content error", http.StatusForbidden)
		}

		return
	}
}

//列表页
func postsHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request form data", 400)
		return
	}

	limit := 5
	p, _ := strconv.Atoi(r.Form.Get("page"))
	if p == 0 {
		p = 1
	}
	start := limit * (p - 1)

	//db
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢數據
	rows, err := db.Query("select id, title, content, create_time from posts where parent_id=0 order by last_reply_time desc limit ?, ?", start, limit)
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
	sess := globalSessions.SessionStart(w, r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request form data", 400)
		return
	}
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
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
	checkErr(err)
	//查詢POST數據
	err = db.QueryRow("select id, title, content, create_time from posts where id=?", post_id).Scan(&id, &title, &content, &create_time)
	switch {
	case err == sql.ErrNoRows:
		log.Printf("No user with that ID.")
		http.NotFound(w, r)
		return
	case err != nil:
		log.Fatal(err)
	default:
		data.Post = &Post{Id: id, Title: title, Content: template.HTML(string(blackfriday.MarkdownCommon([]byte(content)))), Date: time.Unix(int64(create_time), 0).Format("2006-01-02 15:04:05")}
	}
	//查詢reply數據
	replies, err := db.Query("select id, content, create_time from posts where parent_id=? order by id ASC limit ?, ?", post_id, 0, 20)
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

	//浏览数update
	view_key := fmt.Sprintf("post_views:%d", post_id)
	if is_viewed := sess.Get(view_key); is_viewed == nil {
		stmt, err := db.Prepare("update posts SET views=views+1 where id=?")
		checkErr(err)
		res, err := stmt.Exec(post_id)
		checkErr(err)
		affect, err := res.RowsAffected()
		checkErr(err)
		if affect > 0 {
			sess.Set(view_key, true)
		}
	}
}

//编辑
func editHandler(w http.ResponseWriter, r *http.Request) {
	sess := globalSessions.SessionStart(w, r)
	login_uid, _ := strconv.Atoi(fmt.Sprintf("%s", sess.Get("uid")))

	if login_uid == 0 {
		http.Redirect(w, r, "/login", 302)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request form data", 400)
		return
	}
	if r.Method == "GET" {
		post_id, _ := strconv.Atoi(r.Form.Get("post_id"))

		if post_id == 0 {
			http.NotFound(w, r)
			return
		}

		var (
			id      int
			uid     int
			title   string
			content string
		)
		type Data struct {
			Id      int
			Title   string
			Content string
		}
		//db
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
		defer db.Close()
		//查詢數據
		err = db.QueryRow("select id, uid, title, content from posts where id=?", post_id).Scan(&id, &uid, &title, &content)
		switch {
		case err == sql.ErrNoRows:
			log.Printf("No user with that ID.")
			http.NotFound(w, r)
			return
		case err != nil:
			log.Fatal(err)
		default:
			//fmt.Println(uid, login_uid)
			if login_uid != uid {
				http.Error(w, "没有编辑权限", 400)
				return
			}

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
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
		checkErr(err)
		defer db.Close()
		//插入數據
		stmt, err := db.Prepare("update posts SET title=?, content=?, update_time=? where id=? and uid=?")
		checkErr(err)
		res, err := stmt.Exec(title, content, update_time, id, login_uid)
		checkErr(err)
		affect, err := res.RowsAffected()
		checkErr(err)
		if affect > 0 {
			http.Redirect(w, r, "/post?post_id="+strconv.Itoa(id), 302)
			return
		} else {
			http.Error(w, "编辑失败", http.StatusForbidden)
			return
		}

	}
}

//回复
func replyHandler(w http.ResponseWriter, r *http.Request) {
	sess := globalSessions.SessionStart(w, r)
	login_uid, _ := strconv.Atoi(fmt.Sprintf("%s", sess.Get("uid")))

	if login_uid == 0 {
		//http.Error(w, "账号未登录", http.StatusForbidden)
		http.Redirect(w, r, "/login", 302)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid request form data", 400)
		return
	}
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
		http.Error(w, "请输入回复内容", http.StatusForbidden)
		return
	}
	//db
	db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
	checkErr(err)
	defer db.Close()
	//插入數據
	insetReply, err := db.Prepare("INSERT posts SET parent_id=?, uid=?, content=?, create_time=?")
	checkErr(err)
	//更新post replies
	updatePost, err := db.Prepare("update posts set replies = replies + 1, last_reply_time = ?, last_reply_uid = ? where id = ?")
	checkErr(err)
	//开启事务
	tx, err := db.Begin()
	_, err = tx.Stmt(insetReply).Exec(reply_id, login_uid, reply_content, create_time)
	if err != nil {
		fmt.Println("err sql insert reply")
	} else {
		_, err = tx.Stmt(updatePost).Exec(create_time, login_uid, reply_id)
		if err != nil {
			fmt.Println("err sql update post by reply")
		} else {
			tx.Commit()
		}
	}
	if err != nil {
		tx.Rollback()
		http.Error(w, "事务失败", http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/post?post_id="+strconv.Itoa(reply_id), 302)
	return
}

const (
	UsernameMinLen int = 3
	UsernameMaxLen int = 16
)

var (
	UsernameRegexp = regexp.MustCompile(`\A[a-z0-9\-_]{3,16}\z`)
)
var (
	ErrUsernameTooShort = errors.New(`Username is too short`)
	ErrUsernameTooLong  = errors.New(`Username is too long`)
	ErrUsernameInvalid  = errors.New(`Username is not valid`)
)

type Username string

func CheckUsername(uname string) error {
	unameLen := len(uname)

	if unameLen < UsernameMinLen {
		return ErrUsernameTooShort
	}

	if unameLen > UsernameMaxLen {
		return ErrUsernameTooLong
	}

	if !UsernameRegexp.MatchString(uname) {
		return ErrUsernameInvalid
	}

	return nil
}

func regHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//模板應用
		t, _ := template.ParseFiles("html/reg.html")
		t.Execute(w, nil)
	} else {
		r.ParseForm()

		username := r.FormValue("username")
		password := []byte(strings.TrimSpace(r.FormValue("password")))

		if err := CheckUsername(username); err != nil {
			http.Error(w, fmt.Sprintf("%s", err), http.StatusForbidden)
			return
		}

		if len(password) == 0 {
			http.Error(w, "用户名密码不能为空", http.StatusForbidden)
			return
		}
		username = strings.ToLower(username)
		passwd_encode := fmt.Sprintf("%x", md5.Sum(password))
		//db
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
		checkErr(err)
		defer db.Close()
		//查询用户名
		var uid int
		err = db.QueryRow("select uid from users where username=?", username).Scan(&uid)
		if err != sql.ErrNoRows || uid > 0 {
			http.Error(w, "该用户名已存在", http.StatusForbidden)
			return
		}

		reg_time := time.Now().Unix()
		//创建用户
		stmt, err := db.Prepare("INSERT INTO users (`username`, `password`, `reg_time`) value (?,?,?)")
		checkErr(err)
		defer stmt.Close()
		res, err := stmt.Exec(username, passwd_encode, reg_time)
		checkErr(err)
		new_uid, err := res.LastInsertId()
		checkErr(err)
		fmt.Printf("Insert Uid: %d\n", new_uid)
		http.Redirect(w, r, "/login", 302)
		return
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		//模板應用
		t, _ := template.ParseFiles("html/login.html")
		t.Execute(w, nil)
	} else {
		sess := globalSessions.SessionStart(w, r)
		r.ParseForm()
		username := strings.TrimSpace(r.FormValue("username"))
		password := []byte(strings.TrimSpace(r.FormValue("password")))

		if len(username) == 0 || len(password) == 0 {
			http.Error(w, "账号密码不能为空", http.StatusForbidden)
			return
		}
		username = strings.ToLower(username)
		passwd_encode := fmt.Sprintf("%x", md5.Sum(password))
		//db
		db, err := sql.Open("mysql", "admin:1qaz2wsx@tcp(106.186.121.97:3306)/webapp?charset=utf8")
		checkErr(err)
		defer db.Close()
		//查询用户名
		var uid int
		err = db.QueryRow("select uid from users where username=? and password=?", username, passwd_encode).Scan(&uid)
		if err != nil || err == sql.ErrNoRows {
			http.Error(w, "账号错误", http.StatusForbidden)
			return
		}

		//update user info
		stmt, err := db.Prepare("update users SET login_times = login_times + 1, login_time=? where uid=?")
		checkErr(err)
		_, err = stmt.Exec(time.Now().Unix(), uid)
		checkErr(err)

		//set cookie
		expiration := time.Now().AddDate(0, 0, 1)
		cookie := http.Cookie{Name: "uid", Value: strconv.Itoa(uid), Path: "/", HttpOnly: true, Expires: expiration}
		http.SetCookie(w, &cookie)
		sess.Set("uid", strconv.Itoa(uid))
		http.Redirect(w, r, "/posts", 302)
		return
	}
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
