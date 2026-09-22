package handler

import (
	"net/http"
	"time"

	"api-report-nexus/internal/delivery/http/response"
	"api-report-nexus/internal/domain/daypart"
	"api-report-nexus/internal/domain/report"
	"api-report-nexus/internal/usecase/exportexcel"

	"github.com/gin-gonic/gin"
)

type ExportExcel struct {
	Now func() time.Time
}

type storeMeta struct {
	StoreID   string `json:"storeID"`
	StoreName string `json:"storeName"`
	FromDate  string `json:"fromDate"`
	ToDate    string `json:"toDate"`
}

func (h ExportExcel) SalesDayPart(c *gin.Context) {
	var req struct {
		Meta *storeMeta      `json:"meta" binding:"required"`
		Data []daypart.Shift `json:"data" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BindError(c, err)
		return
	}
	tables := daypart.Map(req.Data, req.Meta.FromDate, req.Meta.ToDate, nil)
	name, data, err := exportexcel.SalesDayPart(tables, report.Meta{
		Title:     "Sales by Day Part Report",
		StoreID:   req.Meta.StoreID,
		StoreName: req.Meta.StoreName,
		From:      req.Meta.FromDate,
		To:        req.Meta.ToDate,
	}, h.Now())
	if err != nil {
		response.Fail(c, http.StatusInternalServerError, "Error generating Excel file")
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+name+".xlsx")
	c.Data(http.StatusOK, response.MIMEExcel, data)
}
