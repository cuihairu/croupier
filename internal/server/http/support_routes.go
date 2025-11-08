package httpserver

import (
    "strings"
    "strconv"
    "github.com/gin-gonic/gin"
    sup "github.com/cuihairu/croupier/internal/infra/persistence/gorm/support"
)

// parseUint converts a decimal string id to uint (0 if invalid).
func parseUint(s string) uint {
    if v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64); err == nil { return uint(v) }
    return 0
}

func (s *Server) addSupportRoutes(r *gin.Engine) {
    // Tickets
    r.GET("/api/support/tickets", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:read", "support:manage")
        if !ok { return }
        q := strings.TrimSpace(c.Query("q"))
        status := strings.TrimSpace(c.Query("status"))
        priority := strings.TrimSpace(c.Query("priority"))
        category := strings.TrimSpace(c.Query("category"))
        assignee := strings.TrimSpace(c.Query("assignee"))
        gameID := strings.TrimSpace(c.Query("game_id"))
        env := strings.TrimSpace(c.Query("env"))
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
        if page <= 0 { page = 1 }
        if size <= 0 || size > 200 { size = 20 }
        db := s.gdb.Model(&sup.Ticket{})
        if q != "" { db = db.Where("title LIKE ? OR content LIKE ?", "%"+q+"%", "%"+q+"%") }
        if status != "" { db = db.Where("status = ?", status) }
        if priority != "" { db = db.Where("priority = ?", priority) }
        if category != "" { db = db.Where("category = ?", category) }
        if assignee != "" { db = db.Where("assignee = ?", assignee) }
        if gameID != "" { db = db.Where("game_id = ?", gameID) }
        if env != "" { db = db.Where("env = ?", env) }
        var total int64
        if err := db.Count(&total).Error; err != nil { s.respondError(c, 500, "internal_error", "count failed"); return }
        var arr []sup.Ticket
        if err := db.Order("updated_at DESC").Limit(size).Offset((page-1)*size).Find(&arr).Error; err != nil { s.respondError(c, 500, "internal_error", "list failed"); return }
        out := make([]map[string]any, 0, len(arr))
        for _, t := range arr {
            out = append(out, map[string]any{
                "id": t.ID, "title": t.Title, "category": t.Category, "priority": t.Priority, "status": t.Status,
                "assignee": t.Assignee, "tags": t.Tags, "player_id": t.PlayerID, "contact": t.Contact,
                "game_id": t.GameID, "env": t.Env, "source": t.Source,
                "created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
            })
        }
        // attach request user for audit convenience
        c.Set("user", user)
        s.JSON(c, 200, gin.H{"tickets": out, "total": total, "page": page, "size": size})
    })
    r.POST("/api/support/tickets", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{
            Title, Content, Category, Priority, Status, Assignee, Tags, PlayerID, Contact, GameID, Env, Source string
        }
        if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Title) == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        t := &sup.Ticket{Title: in.Title, Content: in.Content, Category: in.Category, Priority: in.Priority, Status: in.Status, Assignee: in.Assignee, Tags: in.Tags, PlayerID: in.PlayerID, Contact: in.Contact, GameID: in.GameID, Env: in.Env, Source: in.Source}
        if t.Status == "" { t.Status = "open" }
        if t.Priority == "" { t.Priority = "normal" }
        if err := s.gdb.Create(t).Error; err != nil { s.respondError(c, 500, "internal_error", "create failed"); return }
        if s.audit != nil { _ = s.audit.Log("support.ticket_create", user, strconv.FormatUint(uint64(t.ID),10), map[string]string{"ip": c.ClientIP(), "title": t.Title}) }
        s.JSON(c, 201, gin.H{"id": t.ID})
    })
    r.GET("/api/support/tickets/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:read", "support:manage")
        if !ok { return }
        var t sup.Ticket
        if err := s.gdb.First(&t, c.Param("id")).Error; err != nil { s.respondError(c, 404, "not_found", "not found"); return }
        s.JSON(c, 200, t)
    })
    r.PUT("/api/support/tickets/:id", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Title, Content, Category, Priority, Status, Assignee, Tags string }
        if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        var t sup.Ticket
        if err := s.gdb.First(&t, c.Param("id")).Error; err != nil { s.respondError(c, 404, "not_found", "not found"); return }
        if in.Title != "" { t.Title = in.Title }
        if in.Content != "" { t.Content = in.Content }
        if in.Category != "" { t.Category = in.Category }
        if in.Priority != "" { t.Priority = in.Priority }
        if in.Status != "" { t.Status = in.Status }
        if in.Assignee != "" { t.Assignee = in.Assignee }
        if in.Tags != "" || in.Tags == "" { t.Tags = in.Tags }
        if err := s.gdb.Save(&t).Error; err != nil { s.respondError(c, 500, "internal_error", "update failed"); return }
        if s.audit != nil { _ = s.audit.Log("support.ticket_update", user, strconv.FormatUint(uint64(t.ID),10), map[string]string{"ip": c.ClientIP(), "status": t.Status, "assignee": t.Assignee}) }
        c.Status(204)
    })
    r.DELETE("/api/support/tickets/:id", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:manage")
        if !ok { return }
        if err := s.gdb.Delete(&sup.Ticket{}, c.Param("id")).Error; err != nil { s.respondError(c, 500, "internal_error", "delete failed"); return }
        if s.audit != nil { _ = s.audit.Log("support.ticket_delete", user, c.Param("id"), map[string]string{"ip": c.ClientIP()}) }
        c.Status(204)
    })

    // Ticket comments
    r.GET("/api/support/tickets/:id/comments", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:read", "support:manage")
        if !ok { return }
        var arr []sup.TicketComment
        if err := s.gdb.Where("ticket_id = ?", c.Param("id")).Order("created_at ASC").Find(&arr).Error; err != nil { s.respondError(c, 500, "internal_error", "list failed"); return }
        s.JSON(c, 200, gin.H{"comments": arr})
    })
    r.POST("/api/support/tickets/:id/comments", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Content, Attach string }
        if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Content) == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        cm := &sup.TicketComment{TicketID: parseUint(c.Param("id")), Author: user, Content: in.Content, Attach: in.Attach}
        if err := s.gdb.Create(cm).Error; err != nil { s.respondError(c, 500, "internal_error", "create failed"); return }
        if s.audit != nil { _ = s.audit.Log("support.ticket_comment", user, c.Param("id"), map[string]string{"ip": c.ClientIP()}) }
        s.JSON(c, 201, gin.H{"id": cm.ID})
    })

    // Ticket status transition
    r.POST("/api/support/tickets/:id/transition", func(c *gin.Context) {
        user, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Status, Comment string }
        if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Status) == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        var t sup.Ticket
        if err := s.gdb.First(&t, c.Param("id")).Error; err != nil { s.respondError(c, 404, "not_found", "not found"); return }
        from := t.Status
        t.Status = in.Status
        if err := s.gdb.Save(&t).Error; err != nil { s.respondError(c, 500, "internal_error", "update failed"); return }
        if strings.TrimSpace(in.Comment) != "" {
            _ = s.gdb.Create(&sup.TicketComment{TicketID: t.ID, Author: user, Content: in.Comment}).Error
        }
        if s.audit != nil { _ = s.audit.Log("support.ticket_transition", user, strconv.FormatUint(uint64(t.ID),10), map[string]string{"ip": c.ClientIP(), "from": from, "to": t.Status}) }
        c.Status(204)
    })

    // FAQ
    r.GET("/api/support/faq", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:read", "support:manage")
        if !ok { return }
        q := strings.TrimSpace(c.Query("q"))
        category := strings.TrimSpace(c.Query("category"))
        visible := strings.TrimSpace(c.Query("visible"))
        db := s.gdb.Model(&sup.FAQ{})
        if q != "" { db = db.Where("question LIKE ? OR answer LIKE ?", "%"+q+"%", "%"+q+"%") }
        if category != "" { db = db.Where("category = ?", category) }
        if visible != "" {
            if visible == "true" || visible == "1" { db = db.Where("visible = ?", true) } else { db = db.Where("visible = ?", false) }
        }
        var arr []sup.FAQ
        if err := db.Order("sort DESC, updated_at DESC").Find(&arr).Error; err != nil { s.respondError(c, 500, "internal_error", "list failed"); return }
        s.JSON(c, 200, gin.H{"faq": arr})
    })
    r.POST("/api/support/faq", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Question, Answer, Category, Tags string; Visible *bool; Sort *int }
        if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Question) == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        f := &sup.FAQ{Question: in.Question, Answer: in.Answer, Category: in.Category, Tags: in.Tags}
        if in.Visible != nil { f.Visible = *in.Visible }
        if in.Sort != nil { f.Sort = *in.Sort }
        if err := s.gdb.Create(f).Error; err != nil { s.respondError(c, 500, "internal_error", "create failed"); return }
        s.JSON(c, 201, gin.H{"id": f.ID})
    })
    r.PUT("/api/support/faq/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Question, Answer, Category, Tags string; Visible *bool; Sort *int }
        if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        var f sup.FAQ
        if err := s.gdb.First(&f, c.Param("id")).Error; err != nil { s.respondError(c, 404, "not_found", "not found"); return }
        if in.Question != "" { f.Question = in.Question }
        if in.Answer != "" { f.Answer = in.Answer }
        if in.Category != "" || in.Category == "" { f.Category = in.Category }
        if in.Tags != "" || in.Tags == "" { f.Tags = in.Tags }
        if in.Visible != nil { f.Visible = *in.Visible }
        if in.Sort != nil { f.Sort = *in.Sort }
        if err := s.gdb.Save(&f).Error; err != nil { s.respondError(c, 500, "internal_error", "update failed"); return }
        c.Status(204)
    })
    r.DELETE("/api/support/faq/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        if err := s.gdb.Delete(&sup.FAQ{}, c.Param("id")).Error; err != nil { s.respondError(c, 500, "internal_error", "delete failed"); return }
        c.Status(204)
    })

    // Feedback
    r.GET("/api/support/feedback", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:read", "support:manage")
        if !ok { return }
        q := strings.TrimSpace(c.Query("q"))
        category := strings.TrimSpace(c.Query("category"))
        status := strings.TrimSpace(c.Query("status"))
        gameID := strings.TrimSpace(c.Query("game_id"))
        env := strings.TrimSpace(c.Query("env"))
        page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
        size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
        if page <= 0 { page = 1 }
        if size <= 0 || size > 200 { size = 20 }
        db := s.gdb.Model(&sup.Feedback{})
        if q != "" { db = db.Where("content LIKE ?", "%"+q+"%") }
        if category != "" { db = db.Where("category = ?", category) }
        if status != "" { db = db.Where("status = ?", status) }
        if gameID != "" { db = db.Where("game_id = ?", gameID) }
        if env != "" { db = db.Where("env = ?", env) }
        var total int64
        if err := db.Count(&total).Error; err != nil { s.respondError(c, 500, "internal_error", "count failed"); return }
        var arr []sup.Feedback
        if err := db.Order("updated_at DESC").Limit(size).Offset((page-1)*size).Find(&arr).Error; err != nil { s.respondError(c, 500, "internal_error", "list failed"); return }
        s.JSON(c, 200, gin.H{"feedback": arr, "total": total, "page": page, "size": size})
    })
    r.POST("/api/support/feedback", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ PlayerID, Contact, Content, Category, Priority, Status, Attach, GameID, Env string }
        if err := c.BindJSON(&in); err != nil || strings.TrimSpace(in.Content) == "" { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        f := &sup.Feedback{PlayerID: in.PlayerID, Contact: in.Contact, Content: in.Content, Category: in.Category, Priority: in.Priority, Status: in.Status, Attach: in.Attach, GameID: in.GameID, Env: in.Env}
        if f.Status == "" { f.Status = "new" }
        if f.Priority == "" { f.Priority = "normal" }
        if err := s.gdb.Create(f).Error; err != nil { s.respondError(c, 500, "internal_error", "create failed"); return }
        s.JSON(c, 201, gin.H{"id": f.ID})
    })
    r.PUT("/api/support/feedback/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        var in struct{ Category, Priority, Status, Attach string }
        if err := c.BindJSON(&in); err != nil { s.respondError(c, 400, "bad_request", "invalid payload"); return }
        var f sup.Feedback
        if err := s.gdb.First(&f, c.Param("id")).Error; err != nil { s.respondError(c, 404, "not_found", "not found"); return }
        if in.Category != "" || in.Category == "" { f.Category = in.Category }
        if in.Priority != "" { f.Priority = in.Priority }
        if in.Status != "" { f.Status = in.Status }
        if in.Attach != "" || in.Attach == "" { f.Attach = in.Attach }
        if err := s.gdb.Save(&f).Error; err != nil { s.respondError(c, 500, "internal_error", "update failed"); return }
        c.Status(204)
    })
    r.DELETE("/api/support/feedback/:id", func(c *gin.Context) {
        _, _, ok := s.require(c, "support:manage")
        if !ok { return }
        if err := s.gdb.Delete(&sup.Feedback{}, c.Param("id")).Error; err != nil { s.respondError(c, 500, "internal_error", "delete failed"); return }
        c.Status(204)
    })
}
