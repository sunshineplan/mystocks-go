package main

import (
	"log"
	"strings"

	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
)

func showStock(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	index := c.Param("index")
	code := c.Param("code")
	stock := initStock(index, code)
	if stock != nil {
		c.HTML(200, "chart.html", gin.H{"user": username, "index": index, "code": code})
		return
	}
	c.HTML(200, "chart.html", gin.H{"user": username, "index": "n/a", "code": "n/a"})
}

func myStocks(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		userID = 0
	}

	rows, err := db.Query(`SELECT idx, code FROM stock WHERE user_id = ? ORDER BY seq`, userID)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			restore("")
			c.String(501, "")
			return
		}
		log.Println("Failed to get all stocks:", err)
		c.String(500, "")
		return
	}
	defer rows.Close()
	var stocks []stock
	for rows.Next() {
		var index, code string
		if err := rows.Scan(&index, &code); err != nil {
			log.Println("Failed to scan all stocks:", err)
			c.String(500, "")
			return
		}
		stock := initStock(index, code)
		stocks = append(stocks, stock)
	}
	c.JSON(200, doGetRealtimes(stocks))
}

func indices(c *gin.Context) {
	indices := doGetRealtimes([]stock{&sse{Code: "000001"}, &szse{Code: "399001"}, &szse{Code: "399006"}, &szse{Code: "399005"}})
	c.JSON(200, gin.H{"沪": indices[0], "深": indices[1], "创": indices[2], "中小板": indices[3]})
}

func getStock(c *gin.Context) {
	index := c.Query("index")
	code := c.Query("code")
	q := c.Query("q")

	stock := initStock(index, code)

	if q == "realtime" {
		realtime := stock.realtime()
		c.JSON(200, realtime)
		return
	} else if q == "chart" {
		chart := stock.chart()
		c.JSON(200, chart)
		return
	}
	c.String(400, "")
}

func getSuggest(c *gin.Context) {
	keyword := c.Query("keyword")
	sse := sseSuggest(keyword)
	szse := szseSuggest(keyword)
	c.JSON(200, append(sse, szse...))
}

func star(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")
	refer := strings.Split(c.Request.Referer(), "/")
	index := refer[len(refer)-2]
	code := refer[len(refer)-1]

	if userID != nil {
		var exist string
		if err := db.QueryRow(
			"SELECT idx FROM stock WHERE idx = ? AND code = ? AND user_id = ?", index, code, userID).Scan(&exist); err == nil {
			c.String(200, "True")
			return
		}
	}
	c.String(200, "False")
}

func doStar(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")
	refer := strings.Split(c.Request.Referer(), "/")
	index := refer[len(refer)-2]
	code := refer[len(refer)-1]
	action := c.PostForm("action")

	if userID != nil {
		if action == "unstar" {
			if _, err := db.Exec("DELETE FROM stock WHERE idx = ? AND code = ? AND user_id = ?", index, code, userID); err != nil {
				log.Println("Failed to unstar stock:", err)
				c.String(500, "")
				return
			}
		} else {
			if _, err := db.Exec("INSERT INTO stock (idx, code, user_id) VALUES (?, ?, ?)", index, code, userID); err != nil {
				log.Println("Failed to star stock:", err)
				c.String(500, "")
				return
			}
		}
		c.String(200, "1")
		return
	}
	c.String(200, "0")
}

func reorder(c *gin.Context) {
	db, err := getDB()
	if err != nil {
		log.Println("Failed to connect to database:", err)
		c.String(503, "")
		return
	}
	defer db.Close()

	session := sessions.Default(c)
	userID := session.Get("user_id")

	orig := strings.Split(c.PostForm("orig"), " ")
	dest := c.PostForm("dest")

	var origSeq, destSeq int

	if err := db.QueryRow(
		"SELECT seq FROM stock WHERE idx = ? AND code = ? AND user_id = ?", orig[0], orig[1], userID).Scan(&origSeq); err != nil {
		log.Println("Failed to scan orig seq:", err)
		c.String(500, "")
		return
	}
	if dest != "#TOP_POSITION#" {
		d := strings.Split(dest, " ")
		if err := db.QueryRow(
			"SELECT seq FROM stock WHERE idx = ? AND code = ? AND user_id = ?", d[0], d[1], userID).Scan(&destSeq); err != nil {
			log.Println("Failed to scan dest seq:", err)
			c.String(500, "")
			return
		}
	} else {
		destSeq = 0
	}

	if origSeq > destSeq {
		destSeq++
		_, err = db.Exec("UPDATE stock SET seq = seq + 1 WHERE seq >= ? AND user_id = ? AND seq < ?", destSeq, userID, origSeq)
	} else {
		_, err = db.Exec("UPDATE stock SET seq = seq - 1 WHERE seq <= ? AND user_id = ? AND seq > ?", destSeq, userID, origSeq)
	}
	if err != nil {
		log.Println("Failed to update other seq:", err)
		c.String(500, "")
		return
	}
	if _, err := db.Exec(
		"UPDATE stock SET seq = ? WHERE idx = ? AND code = ? AND user_id = ?", destSeq, orig[0], orig[1], userID); err != nil {
		log.Println("Failed to update orig seq:", err)
		c.String(500, "")
		return
	}
	c.String(200, "1")
}
