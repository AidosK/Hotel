package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/erfidev/hotel-web-app/config"
	"github.com/erfidev/hotel-web-app/driver"
	"github.com/erfidev/hotel-web-app/forms"
	"github.com/erfidev/hotel-web-app/models"
	"github.com/erfidev/hotel-web-app/repository"
	"github.com/erfidev/hotel-web-app/repository/dbrepo"
	"github.com/erfidev/hotel-web-app/utils"
	"github.com/go-chi/chi"
)

var Repo *Repository

type Repository struct {
	App *config.AppConfig
	DB  repository.DatabaseRepository
}

func NewRepository(app *config.AppConfig, db *driver.DB) *Repository {
	return &Repository{
		App: app,
		DB:  dbrepo.NewPostgresRepo(db.SQL, app),
	}
}

func NewTestRepository(app *config.AppConfig) *Repository {
	return &Repository{
		App: app,
	}
}

func SetRepo(rep *Repository) {
	Repo = rep
}

func (r Repository) Home(res http.ResponseWriter, req *http.Request) {
	data := make(map[string]interface{})
	data["path"] = "/"
	data["title"] = "Home Page"

	utils.RenderTemplate(res, req, "landing.page.gohtml", &models.TmpData{
		Data: data,
	})
}

func (r Repository) About(res http.ResponseWriter, req *http.Request) {
	utils.RenderTemplate(res, req, "about.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"path":  "/about",
			"title": "about page",
		},
	})
}

func (r Repository) Rooms(res http.ResponseWriter, req *http.Request) {
	pageData := models.TmpData{
		Data: map[string]interface{}{
			"title": "Rooms page",
			"path":  "/rooms",
		},
	}

	switch req.RequestURI {
	case "/rooms":
		utils.RenderTemplate(res, req, "rooms.page.gohtml", &pageData)

	case "/rooms/generals":
		pageData.Data["title"] = "Generals quarters"
		utils.RenderTemplate(res, req, "generals.page.gohtml", &pageData)

	case "/rooms/majors":
		pageData.Data["title"] = "Majors suite"
		utils.RenderTemplate(res, req, "majors.page.gohtml", &pageData)

	default:
		res.Write([]byte("page not found"))
	}
}

func (r Repository) BookNow(res http.ResponseWriter, req *http.Request) {
	pageData := models.TmpData{
		Data: map[string]interface{}{
			"title": "Book now",
			"path":  "/book-now",
		},
		Form: forms.New(nil),
	}

	utils.RenderTemplate(res, req, "book.page.gohtml", &pageData)
}

func (r Repository) Contact(res http.ResponseWriter, req *http.Request) {
	utils.RenderTemplate(res, req, "contact.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"title": "contact page",
			"path":  "/contact",
		},
	})
}

func (r Repository) MakeReservation(res http.ResponseWriter, req *http.Request) {
	reservation, ok := r.App.Session.Get(req.Context(), "reservation").(models.Reservation)
	if !ok {
		http.Redirect(res, req, "/book-now", http.StatusTemporaryRedirect)
		return
	}

	st := reservation.StartDate.Format("2006-01-02")
	ed := reservation.EndDate.Format("2006-01-02")

	room, err := r.DB.FindRoomById(reservation.RoomId)
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	reservation.Room = room

	r.App.Session.Put(req.Context(), "reservation", reservation)

	Data := map[string]interface{}{
		"title":       "make reservation",
		"path":        "/book-now",
		"reservation": reservation,
		"startDate":   st,
		"endDate":     ed,
	}

	utils.RenderTemplate(res, req, "make-reservation.page.gohtml", &models.TmpData{
		Data: Data,
		Form: forms.New(nil),
	})
}

func (r Repository) BookNowPost(res http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	newBookNow := models.BookNow{
		Start: req.Form.Get("start-date"),
		End:   req.Form.Get("ending-date"),
	}

	form := forms.New(req.PostForm)

	// check for valid date's
	form.Has("start-date", req)
	form.Has("ending-date", req)

	// last input check
	if !form.Valid() {
		data := models.TmpData{
			Data: map[string]interface{}{
				"reservation": newBookNow,
				"path":        "/book-now",
				"title":       "Book now",
			},
			Form: form,
		}

		utils.RenderTemplate(res, req, "book.page.gohtml", &data)
		return
	} else {
		layout := "2006-01-02"
		startDate, err := time.Parse(layout, newBookNow.Start)
		if err != nil {
			utils.ServerError(res, err)
			return
		}
		endDate, err := time.Parse(layout, newBookNow.End)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		rooms, err := r.DB.SearchAvailabilityForAllRooms(startDate, endDate)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		if len(rooms) <= 0 {
			r.App.Session.Put(req.Context(), "error", "not availability")
			http.Redirect(res, req, "/book-now", http.StatusSeeOther)
			return
		}

		data := make(map[string]interface{})
		data["title"] = "Choose a room"
		data["path"] = "/book-now"
		data["rooms"] = rooms

		reservation := models.Reservation{
			StartDate: startDate,
			EndDate:   endDate,
		}

		r.App.Session.Put(req.Context(), "reservation", reservation)

		utils.RenderTemplate(res, req, "choose-room.page.gohtml", &models.TmpData{
			Data: data,
		})
	}
}

func (r Repository) ChooseRoom(res http.ResponseWriter, req *http.Request) {
	roomId, err := strconv.Atoi(chi.URLParam(req, "id"))
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	reservation := r.App.Session.Get(req.Context(), "reservation").(models.Reservation)

	reservation.RoomId = roomId

	r.App.Session.Put(req.Context(), "reservation", reservation)
	http.Redirect(res, req, "/make-reservation", http.StatusTemporaryRedirect)
}

func (r Repository) MakeReservationPost(res http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	reservation := r.App.Session.Get(req.Context(), "reservation").(models.Reservation)

	reservation.FirstName = req.Form.Get("first_name")
	reservation.LastName = req.Form.Get("last_name")
	reservation.Email = req.Form.Get("email")
	reservation.Phone = req.Form.Get("phone")

	form := forms.New(req.PostForm)

	form.Has("first_name", req)
	form.Has("last_name", req)
	form.Has("email", req)
	form.Has("phone", req)

	if !form.Valid() {
		data := make(map[string]interface{})
		data["reservation"] = reservation
		data["title"] = "Make reservation"
		data["path"] = "/book-now"

		utils.RenderTemplate(res, req, "make-reservation.page.gohtml", &models.TmpData{
			Data: data,
			Form: form,
		})

		return
	}

	reservationId, errInsert := r.DB.InsertReservation(reservation)
	if errInsert != nil {
		utils.ServerError(res, errInsert)
		return
	}

	roomRestrictions := models.RoomRestriction{
		StartDate:     reservation.StartDate,
		EndDate:       reservation.EndDate,
		ReservationId: reservationId,
		RestrictionId: 1,
		RoomId:        reservation.RoomId,
	}

	err = r.DB.InsertRoomRestriction(roomRestrictions)
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	r.App.Session.Put(req.Context(), "reservation", reservation)

	// sending confirmation email
	emailContent := `
	<h1>hi</h1>
	<p>email from 'test' hotel</p>
	<p>your reservation has complete enjoy it!</p>
`
	newEmail := models.MailData{
		From:    os.Getenv("EMAIL"),
		To:      reservation.Email,
		Pass:    os.Getenv("EMAIL_PASS"),
		Subject: "Reservation confirmation",
		Content: emailContent,
	}

	r.App.MailChan <- newEmail

	http.Redirect(res, req, "/reservation-summary", http.StatusSeeOther)
}

func (r Repository) ReservationSummary(res http.ResponseWriter, req *http.Request) {
	reservation, isOk := r.App.Session.Get(req.Context(), "reservation").(models.Reservation)
	if !isOk {
		r.App.Session.Put(req.Context(), "error", "can't get reservation data from session")
		http.Redirect(res, req, "/", http.StatusTemporaryRedirect)
		return
	}

	r.App.Session.Remove(req.Context(), "reservation")

	utils.RenderTemplate(res, req, "reservation.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"reservation": reservation,
			"title":       "Reservation summary",
			"path":        "/book-now",
		},
	})
}

func (r Repository) SearchAvailability(res http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	startDate := req.Form.Get("start-date")
	endDate := req.Form.Get("ending-date")
	roomId := req.Form.Get("room_id")

	form := forms.New(req.PostForm)

	form.Has("start-date", req)
	form.Has("ending-date", req)

	rawResponse := make(map[string]interface{})

	if !form.Valid() {
		rawResponse["status"] = 406
		rawResponse["msg"] = "please enter a valid date"

		toJson, err := json.Marshal(rawResponse)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		res.Write(toJson)
	} else {
		roomIdInt, _ := strconv.Atoi(roomId)
		stDateToTime, _ := time.Parse("2006-01-02", startDate)
		edDateToTime, _ := time.Parse("2006-01-02", endDate)

		response, err := r.DB.SearchAvailability(roomIdInt, stDateToTime, edDateToTime)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		if response {
			reservation := models.Reservation{
				StartDate: stDateToTime,
				EndDate:   edDateToTime,
			}
			r.App.Session.Put(req.Context(), "reservation", reservation)

			rawResponse["msg"] = "room is available!"
			rawResponse["status"] = 200
			rawResponse["roomID"] = roomIdInt

			toJson, _ := json.Marshal(rawResponse)
			res.Write(toJson)
		} else {
			rawResponse["status"] = 404
			rawResponse["msg"] = "this room is unavailable!"

			toJson, err := json.Marshal(rawResponse)
			if err != nil {
				utils.ServerError(res, err)
				return
			}

			res.Write(toJson)
		}
	}
}
func (r Repository) SignUp(res http.ResponseWriter, req *http.Request) {
	utils.RenderTemplate(res, req, "signup.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"path":  "/signup",
			"title": "SignUp your account",
		},
		Form: forms.New(nil),
	})
}
func (r Repository) SignUpPost(res http.ResponseWriter, req *http.Request) {
	_ = r.App.Session.RenewToken(req.Context())

	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	user := models.User{
		FirstName:   req.Form.Get("first_name"),
		LastName:    req.Form.Get("last_name"),
		Email:       req.Form.Get("email"),
		Password:    req.Form.Get("password"),
		AccessLevel: 1,
	}

	form := forms.New(req.PostForm)
	form.Required("first_name", "last_name", "email", "password")
	form.MinLength("password", 4)

	if !form.Valid() {
		data := make(map[string]interface{})
		data["title"] = "Sign Up"
		data["path"] = "/signup"
		data["user"] = user

		utils.RenderTemplate(res, req, "signup.page.gohtml", &models.TmpData{
			Data: data,
			Form: form,
		})
		return
	}
	_, err = r.DB.InsertUser(user)
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	rawJson := make(map[string]interface{})
	rawJson["msg"] = "Sign Up successful"
	rawJson["status"] = 200

	toJson, _ := json.Marshal(rawJson)
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	res.Write(toJson)
}

func (r Repository) Login(res http.ResponseWriter, req *http.Request) {
	utils.RenderTemplate(res, req, "login.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"path":  "/login",
			"title": "login in account",
		},
		Form: forms.New(nil),
	})
}

func (r Repository) LoginPost(res http.ResponseWriter, req *http.Request) {
	_ = r.App.Session.RenewToken(req.Context())

	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	user := models.User{
		Email:       req.Form.Get("email"),
		Password:    req.Form.Get("password"),
		AccessLevel: 1,
	}

	form := forms.New(req.PostForm)

	form.Has("email", req)
	form.Has("password", req)

	if !form.Valid() {
		data := make(map[string]interface{})
		data["title"] = "login in account"
		data["path"] = "/login"
		data["user"] = user

		utils.RenderTemplate(res, req, "login.page.gohtml", &models.TmpData{
			Data: data,
			Form: form,
		})
		return
	} else {
		rawJson := make(map[string]interface{})
		// authenticate
		id, authenticate, err := r.DB.Authenticate(user.Email, user.Password)
		if !authenticate {
			rawJson["msg"] = err
			rawJson["status"] = 403
			toJson, _ := json.Marshal(rawJson)
			res.Header().Add("Content-Type", "application/json; charset=utf8")
			res.Write(toJson)
		} else {
			rawJson["msg"] = "login successful"
			rawJson["status"] = 200
			toJson, _ := json.Marshal(rawJson)
			res.Header().Add("Content-Type", "application/json; charset=utf8")
			res.Write(toJson)
			r.App.Session.Put(req.Context(), "user_id", id)
		}
	}
}

func (r Repository) Logout(res http.ResponseWriter, req *http.Request) {
	_ = r.App.Session.Destroy(req.Context())
	_ = r.App.Session.RenewToken(req.Context())

	http.Redirect(res, req, "/", http.StatusSeeOther)
}

func (r Repository) AdminDashboard(res http.ResponseWriter, req *http.Request) {
	// Check if the user has access_level = 2

	// Render the admin dashboard if the access check passes
	utils.RenderTemplate(res, req, "admin-dashboard.page.gohtml", &models.TmpData{
		Data: map[string]interface{}{
			"title": "Admin dashboard",
			"path":  "/dashboard",
		},
	})
}

func (r Repository) AllReservationsApi(res http.ResponseWriter, req *http.Request) {
	adminAuth := utils.IsAuthenticated(req)

	jsonTmp := make(map[string]interface{})

	if !adminAuth {
		jsonTmp["msg"] = "you are not allow to access this api!"
		jsonTmp["status"] = http.StatusNotAcceptable
		toJson, _ := json.Marshal(jsonTmp)

		res.Header().Add("Content-Type", "application/json; charset=utf8")
		res.Write(toJson)

		return
	}

	reservations, err := r.DB.AllReservations()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	toJson, _ := json.Marshal(reservations)

	res.Header().Add("Content-Type", "application/json; charset=utf8")
	res.Write(toJson)
}

func (r Repository) AdminReservations(res http.ResponseWriter, req *http.Request) {
	reservations, err := r.DB.AllReservations()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	data := make(map[string]interface{})
	data["reservations"] = reservations
	data["title"] = "all reservations"
	data["path"] = "/reservations"

	utils.RenderTemplate(res, req, "admin-reservations.page.gohtml", &models.TmpData{
		Data: data,
	})
}

func (r Repository) NewReservations(res http.ResponseWriter, req *http.Request) {
	reservations, err := r.DB.AllNewReservations()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	data := make(map[string]interface{})
	data["reservations"] = reservations
	data["title"] = "New reservations"
	data["path"] = "/newReservations"

	utils.RenderTemplate(res, req, "admin-newReservations.page.gohtml", &models.TmpData{
		Data: data,
	})

}

func (r Repository) Admin(res http.ResponseWriter, req *http.Request) {
	http.Redirect(res, req, "/admin/dashboard", http.StatusSeeOther)
}

func (r Repository) SingleReservation(res http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(req, "id"))
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	getReservation, errDb := r.DB.GetReservationById(id)
	if errDb != nil {
		utils.ServerError(res, errDb)
		return
	}

	data := make(map[string]interface{})
	data["reservation"] = getReservation
	data["title"] = fmt.Sprintf("%s Reservation", getReservation.Email)
	data["path"] = "/reservations"
	data["id"] = id

	utils.RenderTemplate(res, req, "admin-single-res.page.gohtml", &models.TmpData{
		Data: data,
		Form: forms.New(nil),
	})
}

func (r Repository) ApiUpdateReservation(res http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	form := forms.New(req.PostForm)

	form.Has("email", req)
	form.Has("first_name", req)
	form.Has("last_name", req)
	form.Has("phone", req)

	idInt, _ := strconv.Atoi(req.Form.Get("id"))
	reservation := models.Reservation{
		Email:     req.Form.Get("email"),
		FirstName: req.Form.Get("first_name"),
		LastName:  req.Form.Get("last_name"),
		Phone:     req.Form.Get("phone"),
		ID:        idInt,
	}

	data := make(map[string]interface{})

	if !form.Valid() {
		data["reservation"] = reservation
		data["title"] = "Update reservation"
		data["path"] = "/reservations"

		r.App.Session.Put(req.Context(), "error", "please fill correct input value!")
		utils.RenderTemplate(res, req, "admin-single-res.page.gohtml", &models.TmpData{
			Data: data,
			Form: form,
		})
		return
	}

	result, errDb := r.DB.UpdateReservation(reservation)
	if errDb != nil {
		utils.ServerError(res, errDb)
		return
	} else {
		if !result {
			utils.ServerError(res, errors.New("we have problem on server!"))
			return
		}

		r.App.Session.Put(req.Context(), "flash", "update reservation success!")
		http.Redirect(res, req, "/admin/reservations", http.StatusSeeOther)
	}
}

func (r Repository) DeleteReservation(res http.ResponseWriter, req *http.Request) {
	id, _ := strconv.Atoi(req.Form.Get("id"))

	deleting, _ := r.DB.DeleteReservation(id)
	if !deleting {
		utils.ServerError(res, errors.New("we have problem on server!"))
		return
	}

	jsonTmp := make(map[string]interface{})

	jsonTmp["status"] = 200
	jsonTmp["msg"] = "reservation deleted!"

	toJson, err := json.Marshal(jsonTmp)
	if err != nil {
		utils.ServerError(res, errors.New("we have problem on server!"))
		return
	}

	res.Header().Add("Content-Type", "application/json; charset=utf8")
	res.Write(toJson)
}

func (r Repository) CompleteReservation(res http.ResponseWriter, req *http.Request) {
	id, err := strconv.Atoi(req.Form.Get("id"))
	if err != nil {
		utils.ServerError(res, err)
		return
	}

	jsonTmp := make(map[string]interface{})

	result, _ := r.DB.CompleteReservation(id)
	if !result {
		jsonTmp["status"] = http.StatusConflict
		jsonTmp["msg"] = "we have problem in server!"

		toJson, err := json.Marshal(jsonTmp)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		res.Header().Add("Content-Type", "application/json; charset=utf8")
		res.Write(toJson)
	} else {
		jsonTmp["status"] = 200
		jsonTmp["msg"] = "reservation processed complete!"
		toJson, err := json.Marshal(jsonTmp)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		res.Header().Add("Content-Type", "application/json; charset=utf8")
		res.Write(toJson)
	}
}

func (r Repository) AdminReservationCelendar(res http.ResponseWriter, req *http.Request) {
	now := time.Now()

	stringMap := make(map[string]string)
	data := make(map[string]interface{})

	if req.URL.Query().Get("y") != "" && req.URL.Query().Get("m") != "" {
		year, _ := strconv.Atoi(req.URL.Query().Get("y"))
		month, _ := strconv.Atoi(req.URL.Query().Get("m"))
		now = time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

		next := now.AddDate(0, 1, 0)
		last := now.AddDate(0, -1, 0)

		nextMonth := next.Format("01")
		nextYear := next.Format("2006")
		lastMonth := last.Format("01")
		lastYear := last.Format("2006")

		stringMap["next_month"] = nextMonth
		stringMap["next_year"] = nextYear
		stringMap["last_month"] = lastMonth
		stringMap["last_year"] = lastYear
		stringMap["now_month"] = now.Format("01")
		stringMap["now_year"] = now.Format("2006")

		data["title"] = "Reservations Celendar"
		data["path"] = "/reservation-celendar"

		startMonth := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		EndMonth := startMonth.AddDate(0, 1, 0)
		getResByMonth, err := r.DB.GetReservationsBetweemMonth(startMonth, EndMonth)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		timeBetweens := utils.ReturnBetweenDates(getResByMonth)
		toHumanTimeSlice := []string{}
		for _, value := range timeBetweens {
			humanTime := value.Format("02")
			toHumanTimeSlice = append(toHumanTimeSlice, humanTime)
		}
		removeDuplicate := utils.DeleteDuplicateValues(toHumanTimeSlice)
		data["times"] = removeDuplicate

		utils.RenderTemplate(res, req, "admin-res-celendar.page.gohtml", &models.TmpData{
			StringMap: stringMap,
			Data:      data,
		})
	} else {
		next := now.AddDate(0, 1, 0)
		last := now.AddDate(0, -1, 0)

		nextMonth := next.Format("01")
		nextYear := next.Format("2006")
		lastMonth := last.Format("01")
		lastYear := last.Format("2006")

		stringMap["next_month"] = nextMonth
		stringMap["next_year"] = nextYear
		stringMap["last_month"] = lastMonth
		stringMap["last_year"] = lastYear
		stringMap["now_year"] = now.Format("2006")
		stringMap["now_month"] = now.Format("01")
		data["title"] = "Reservations Celendar"
		data["path"] = "/reservations-celendar"

		startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		EndMonth := startMonth.AddDate(0, 1, 0)
		getResByMonth, err := r.DB.GetReservationsBetweemMonth(startMonth, EndMonth)
		if err != nil {
			utils.ServerError(res, err)
			return
		}

		timeBetweens := utils.ReturnBetweenDates(getResByMonth)
		toHumanTimeSlice := []string{}
		for _, value := range timeBetweens {
			humanTime := value.Format("02")
			toHumanTimeSlice = append(toHumanTimeSlice, humanTime)
		}
		removeDuplicate := utils.DeleteDuplicateValues(toHumanTimeSlice)
		data["times"] = removeDuplicate

		utils.RenderTemplate(res, req, "admin-res-celendar.page.gohtml", &models.TmpData{
			StringMap: stringMap,
			Data:      data,
		})
	}
}
