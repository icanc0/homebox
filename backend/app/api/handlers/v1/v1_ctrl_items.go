package v1

import (
	"encoding/csv"
	"net/http"
	"net/url"
	"strconv"

	"github.com/google/uuid"
	"github.com/hay-kot/homebox/backend/internal/core/services"
	"github.com/hay-kot/homebox/backend/internal/data/repo"
	"github.com/hay-kot/homebox/backend/internal/sys/validate"
	"github.com/hay-kot/homebox/backend/pkgs/server"
	"github.com/rs/zerolog/log"
)

// HandleItemsGetAll godoc
// @Summary  Get All Items
// @Tags     Items
// @Produce  json
// @Param    q         query    string   false "search string"
// @Param    page      query    int      false "page number"
// @Param    pageSize  query    int      false "items per page"
// @Param    labels    query    []string false "label Ids"    collectionFormat(multi)
// @Param    locations query    []string false "location Ids" collectionFormat(multi)
// @Success  200       {object} repo.PaginationResult[repo.ItemSummary]{}
// @Router   /v1/items [GET]
// @Security Bearer
func (ctrl *V1Controller) HandleItemsGetAll() server.HandlerFunc {
	uuidList := func(params url.Values, key string) []uuid.UUID {
		var ids []uuid.UUID
		for _, id := range params[key] {
			uid, err := uuid.Parse(id)
			if err != nil {
				continue
			}
			ids = append(ids, uid)
		}
		return ids
	}

	intOrNegativeOne := func(s string) int {
		i, err := strconv.Atoi(s)
		if err != nil {
			return -1
		}
		return i
	}

	getBool := func(s string) bool {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return false
		}
		return b
	}

	extractQuery := func(r *http.Request) repo.ItemQuery {
		params := r.URL.Query()

		return repo.ItemQuery{
			Page:            intOrNegativeOne(params.Get("page")),
			PageSize:        intOrNegativeOne(params.Get("perPage")),
			Search:          params.Get("q"),
			LocationIDs:     uuidList(params, "locations"),
			LabelIDs:        uuidList(params, "labels"),
			IncludeArchived: getBool(params.Get("includeArchived")),
		}
	}

	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := services.NewContext(r.Context())
		items, err := ctrl.repo.Items.QueryByGroup(ctx, ctx.GID, extractQuery(r))
		if err != nil {
			log.Err(err).Msg("failed to get items")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}
		return server.Respond(w, http.StatusOK, items)
	}
}

// HandleItemsCreate godoc
// @Summary  Create a new item
// @Tags     Items
// @Produce  json
// @Param    payload body     repo.ItemCreate true "Item Data"
// @Success  200     {object} repo.ItemSummary
// @Router   /v1/items [POST]
// @Security Bearer
func (ctrl *V1Controller) HandleItemsCreate() server.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		createData := repo.ItemCreate{}
		if err := server.Decode(r, &createData); err != nil {
			log.Err(err).Msg("failed to decode request body")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		user := services.UseUserCtx(r.Context())
		item, err := ctrl.repo.Items.Create(r.Context(), user.GroupID, createData)
		if err != nil {
			log.Err(err).Msg("failed to create item")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.Respond(w, http.StatusCreated, item)
	}
}

// HandleItemGet godocs
// @Summary  Gets a item and fields
// @Tags     Items
// @Produce  json
// @Param    id  path     string true "Item ID"
// @Success  200 {object} repo.ItemOut
// @Router   /v1/items/{id} [GET]
// @Security Bearer
func (ctrl *V1Controller) HandleItemGet() server.HandlerFunc {
	return ctrl.handleItemsGeneral()
}

// HandleItemDelete godocs
// @Summary  deletes a item
// @Tags     Items
// @Produce  json
// @Param    id path string true "Item ID"
// @Success  204
// @Router   /v1/items/{id} [DELETE]
// @Security Bearer
func (ctrl *V1Controller) HandleItemDelete() server.HandlerFunc {
	return ctrl.handleItemsGeneral()
}

// HandleItemUpdate godocs
// @Summary  updates a item
// @Tags     Items
// @Produce  json
// @Param    id      path     string          true "Item ID"
// @Param    payload body     repo.ItemUpdate true "Item Data"
// @Success  200     {object} repo.ItemOut
// @Router   /v1/items/{id} [PUT]
// @Security Bearer
func (ctrl *V1Controller) HandleItemUpdate() server.HandlerFunc {
	return ctrl.handleItemsGeneral()
}

func (ctrl *V1Controller) handleItemsGeneral() server.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		ctx := services.NewContext(r.Context())
		ID, err := ctrl.routeID(r)
		if err != nil {
			return err
		}

		switch r.Method {
		case http.MethodGet:
			items, err := ctrl.repo.Items.GetOneByGroup(r.Context(), ctx.GID, ID)
			if err != nil {
				log.Err(err).Msg("failed to get item")
				return validate.NewRequestError(err, http.StatusInternalServerError)
			}
			return server.Respond(w, http.StatusOK, items)
		case http.MethodDelete:
			err = ctrl.repo.Items.DeleteByGroup(r.Context(), ctx.GID, ID)
			if err != nil {
				log.Err(err).Msg("failed to delete item")
				return validate.NewRequestError(err, http.StatusInternalServerError)
			}
			return server.Respond(w, http.StatusNoContent, nil)
		case http.MethodPut:
			body := repo.ItemUpdate{}
			if err := server.Decode(r, &body); err != nil {
				log.Err(err).Msg("failed to decode request body")
				return validate.NewRequestError(err, http.StatusInternalServerError)
			}
			body.ID = ID
			result, err := ctrl.repo.Items.UpdateByGroup(r.Context(), ctx.GID, body)
			if err != nil {
				log.Err(err).Msg("failed to update item")
				return validate.NewRequestError(err, http.StatusInternalServerError)
			}
			return server.Respond(w, http.StatusOK, result)
		}

		return nil
	}
}

// HandleItemsImport godocs
// @Summary  imports items into the database
// @Tags     Items
// @Produce  json
// @Success  204
// @Param    csv formData file true "Image to upload"
// @Router   /v1/items/import [Post]
// @Security Bearer
func (ctrl *V1Controller) HandleItemsImport() server.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {

		err := r.ParseMultipartForm(ctrl.maxUploadSize << 20)
		if err != nil {
			log.Err(err).Msg("failed to parse multipart form")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		file, _, err := r.FormFile("csv")
		if err != nil {
			log.Err(err).Msg("failed to get file from form")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		reader := csv.NewReader(file)
		data, err := reader.ReadAll()
		if err != nil {
			log.Err(err).Msg("failed to read csv")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		user := services.UseUserCtx(r.Context())

		_, err = ctrl.svc.Items.CsvImport(r.Context(), user.GroupID, data)
		if err != nil {
			log.Err(err).Msg("failed to import items")
			return validate.NewRequestError(err, http.StatusInternalServerError)
		}

		return server.Respond(w, http.StatusNoContent, nil)
	}
}