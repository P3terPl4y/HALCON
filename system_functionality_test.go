package main

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"goravel/app/facades"
	"goravel/app/models"
	"goravel/app/requests"
	"goravel/app/services"
)

func TestSystemFunctionalIntegration(t *testing.T) {
	if os.Getenv("HALCON_SYSTEM_TEST") != "1" {
		t.Skip("opt-in isolated functional suite")
	}
	f := newSystemFixture(t)
	us := services.NewUserService()
	hs := services.NewHalconService()
	actors := make([]*simulatedActor, 6)
	for i, role := range []string{"admin", "moderator", "moderator", "user", "user", "user"} {
		a := f.actor(i)
		a.Email = fmt.Sprintf("system%d@example.test", i)
		a.Password = "SystemTest123!"
		u, err := us.AdminCreateUser(&requests.CreateUserRequest{Name: fmt.Sprintf("Persona %d", i), Email: a.Email, Phone: fmt.Sprintf("system%d", i), Password: a.Password, Role: role})
		require.NoError(t, err)
		a.ID = u.ID
		a.Role = role
		require.NoError(t, a.login())
		p, err := hs.EnsurePersonal(u.ID)
		require.NoError(t, err)
		a.PersonalID = p.ID
		actors[i] = a
	}
	admin, mod, outsider, user, recipient := actors[0], actors[1], actors[2], actors[3], actors[4]
	post := func(a *simulatedActor, path string, v url.Values, status int) {
		t.Helper()
		r, err := a.post(path, v)
		expectResponse(t, r, err, status)
	}
	redirect := func(a *simulatedActor, path string, v url.Values) {
		t.Helper()
		r, err := a.post(path, v)
		expectRedirect(t, r, err)
	}

	t.Run("all_page_routes_and_role_boundaries", func(t *testing.T) {
		for _, path := range []string{"/", "/login", "/register", "/profile", "/profile/edit", "/api/session", "/api/tracking"} {
			r, err := user.request("GET", path, nil)
			expectResponse(t, r, err, 200)
		}
		for _, path := range []string{"/admin/users", "/admin/users/create", "/admin/halcones", "/admin/halcones/create", "/users"} {
			r, err := admin.request("GET", path, nil)
			expectResponse(t, r, err, 200)
		}
		for _, path := range []string{"/moderator/halcones", "/users"} {
			r, err := mod.request("GET", path, nil)
			expectResponse(t, r, err, 200)
		}
		for _, a := range []*simulatedActor{user, mod} {
			r, err := a.request("GET", "/admin/users", nil)
			expectResponse(t, r, err, 403)
		}
		for _, path := range []string{"/users", "/moderator/halcones"} {
			r, err := user.request("GET", path, nil)
			expectResponse(t, r, err, 403)
		}
		r, err := mod.request("GET", "/moderator/halcon", nil)
		expectRedirect(t, r, err)
		require.Equal(t, "/moderator/halcones", r.location)
		anon := f.actor(90)
		r, err = anon.request("GET", "/api/tracking", nil)
		expectResponse(t, r, err, 401)
	})
	t.Run("profile_update_validation_ownership_and_password", func(t *testing.T) {
		for _, values := range []url.Values{{"email": {"not-an-email"}}, {"name": {" "}}, {"password": {"a"}}, {"password": {strings.Repeat("a", 73)}}} {
			post(user, "/profile/edit", values, 422)
		}
		redirect(user, "/profile/edit", url.Values{"name": {" Mi nombre "}, "email": {strings.ToUpper(user.Email)}, "user_id": {fmt.Sprint(recipient.ID)}, "role": {"admin"}, "password": {"NewStrongTest123!"}})
		u, err := us.GetByID(user.ID)
		require.NoError(t, err)
		require.Equal(t, "Mi nombre", u.Name)
		require.Equal(t, user.Email, u.Email)
		require.Equal(t, "user", u.Role)
		require.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.Password), []byte("NewStrongTest123!")))
		other, err := us.GetByID(recipient.ID)
		require.NoError(t, err)
		require.Equal(t, "Persona 4", other.Name)
		user.Password = "NewStrongTest123!"
		require.NoError(t, user.login())
		post(admin, fmt.Sprintf("/admin/users/%d", user.ID), url.Values{"role": {"root"}}, 422)
	})
	t.Run("sharing_apis_and_tenant_isolation", func(t *testing.T) {
		redirect(user, "/profile/share", url.Values{"recipient_email": {strings.ToUpper(recipient.Email)}})
		hsVisible, err := hs.VisibleTo(recipient.ID)
		require.NoError(t, err)
		require.Len(t, hsVisible, 2)
		allowed, err := hs.CanView(user.PersonalID, recipient.ID)
		require.NoError(t, err)
		require.True(t, allowed)
		post(user, "/api/tracking/recipient", url.Values{"recipient_id": {fmt.Sprint(user.ID)}}, 422)
		post(user, "/api/tracking/recipient", url.Values{"recipient_email": {"' OR 1=1 --"}}, 422)
		post(user, "/api/tracking/recipient", url.Values{"recipient_id": {"0"}}, 200)
		hsVisible, err = hs.VisibleTo(recipient.ID)
		require.NoError(t, err)
		require.Len(t, hsVisible, 1)
		allowed, err = hs.CanView(user.PersonalID, recipient.ID)
		require.NoError(t, err)
		require.False(t, allowed)
		allowed, err = hs.CanView(user.PersonalID, admin.ID)
		require.NoError(t, err)
		require.True(t, allowed)
		allowed, err = hs.CanView(999999, admin.ID)
		require.NoError(t, err)
		require.False(t, allowed)
		r, err := user.request("POST", "/api/tracking/recipient", url.Values{"recipient_id": {fmt.Sprint(recipient.ID)}})
		expectResponse(t, r, err, 403)
	})
	t.Run("device_service_crud_queries_history_and_concurrency", func(t *testing.T) {
		_, err := hs.Create(&requests.CreateHalconRequest{Name: "Device", ModeratorID: user.ID})
		require.Error(t, err)
		_, err = hs.Create(&requests.CreateHalconRequest{Name: " ", ModeratorID: mod.ID})
		require.Error(t, err)
		h, err := hs.Create(&requests.CreateHalconRequest{Name: " Dispositivo de prueba ", ModeratorID: mod.ID})
		require.NoError(t, err)
		require.Equal(t, "Dispositivo de prueba", h.Name)
		got, err := hs.ValidateToken(h.Token)
		require.NoError(t, err)
		require.Equal(t, h.ID, got.ID)
		_, err = hs.ValidateToken("wrong")
		require.Error(t, err)
		require.NoError(t, hs.Activate(h.ID))
		got, err = hs.GetByID(h.ID)
		require.NoError(t, err)
		require.Nil(t, got.LastSeen, "activation must not invent GPS 0,0")
		require.NoError(t, hs.Deactivate(h.ID))
		require.Error(t, hs.UpdateLastLocation(h.ID, math.NaN(), 0))
		require.Error(t, hs.UpdateLastLocation(999999, 0, 0))
		require.NoError(t, hs.UpdateLastLocation(h.ID, 0, 0))
		h, err = hs.Update(h.ID, &requests.UpdateHalconRequest{Name: "Nombre actualizado"})
		require.NoError(t, err)
		_, err = hs.AdminUpdateHalcon(h.ID, "Nombre administrativo", false)
		require.NoError(t, err)
		_, err = hs.AdminGetHalcon(h.ID)
		require.NoError(t, err)
		_, err = hs.Assign(h.ID, user.ID, outsider.ID, "")
		require.Error(t, err)
		_, err = hs.Assign(user.PersonalID, recipient.ID, mod.ID, "")
		require.Error(t, err)
		_, err = hs.Assign(h.ID, user.ID, mod.ID, "PKG-1")
		require.NoError(t, err)
		assigned, err := hs.GetByAssignedUserID(user.ID)
		require.NoError(t, err)
		require.Len(t, assigned, 1)
		owned, err := hs.GetByModeratorID(mod.ID)
		require.NoError(t, err)
		require.Len(t, owned, 1)
		require.True(t, hs.CanManage(h.ID, mod.ID))
		require.True(t, hs.CanManage(h.ID, admin.ID))
		require.False(t, hs.CanManage(h.ID, outsider.ID))
		require.False(t, hs.CanManage(user.PersonalID, admin.ID))
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _, e := hs.Assign(h.ID, recipient.ID, mod.ID, "concurrent"); errs <- e }()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			require.NoError(t, e)
		}
		count, err := facades.Orm().Query().Model(&models.HalconAssignment{}).Where("halcon_id = ?", h.ID).Where("ended_at IS NULL").Count()
		require.NoError(t, err)
		require.Equal(t, int64(1), count)
		history, err := hs.AdminGetAssignmentsHistory(h.ID)
		require.NoError(t, err)
		require.Len(t, history, 9)
		require.NotNil(t, history[0].User)
		require.NotNil(t, history[0].Moderator)
		list, total, err := hs.ListByModerator(mod.ID, 1, 20, "administrativo", "0", false)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		require.Equal(t, recipient.ID, list[0].AssignedUser.ID)
		_, total, err = hs.AdminListHalcones(1, 20, "administrativo", "0")
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		stats, err := hs.StatsByModerator(mod.ID, false)
		require.NoError(t, err)
		require.Equal(t, int64(1), stats["assigned"])
		stats, err = hs.AdminCountStats()
		require.NoError(t, err)
		require.Equal(t, int64(1), stats["assigned"])
		_, err = hs.Count()
		require.NoError(t, err)
		require.NoError(t, hs.AdminDeleteHalcon(h.ID))
		_, err = hs.GetByID(h.ID)
		require.Error(t, err)
		history, err = hs.AdminGetAssignmentsHistory(h.ID)
		require.NoError(t, err)
		require.Empty(t, history)
	})
	t.Run("administration_forms_crud_and_user_deletion", func(t *testing.T) {
		redirect(admin, "/admin/users", url.Values{"name": {"Temporal"}, "email": {"temp@example.test"}, "phone": {"temp-phone"}, "password": {"TemporaryTest123!"}, "role": {"moderator"}})
		var u models.User
		require.NoError(t, facades.Orm().Query().Where("email = ?", "temp@example.test").FirstOrFail(&u))
		r, err := admin.request("GET", fmt.Sprintf("/admin/users/%d/edit", u.ID), nil)
		expectResponse(t, r, err, 200)
		redirect(admin, fmt.Sprintf("/admin/users/%d", u.ID), url.Values{"name": {"Temporal modificado"}, "email": {"TEMP@EXAMPLE.TEST"}, "role": {"moderator"}})
		p, err := hs.EnsurePersonal(u.ID)
		require.NoError(t, err)
		require.NoError(t, hs.SetRecipient(recipient.ID, u.ID))
		device, err := hs.Create(&requests.CreateHalconRequest{Name: "Dispositivo de usuario eliminado", ModeratorID: u.ID})
		require.NoError(t, err)
		_, err = hs.Assign(device.ID, user.ID, u.ID, "history")
		require.NoError(t, err)
		redirect(admin, fmt.Sprintf("/admin/users/%d/delete", u.ID), url.Values{})
		_, err = us.GetByID(u.ID)
		require.Error(t, err)
		_, err = hs.GetByID(p.ID)
		require.Error(t, err)
		device, err = hs.GetByID(device.ID)
		require.NoError(t, err)
		require.Zero(t, device.ModeratorID)
		p, err = hs.EnsurePersonal(recipient.ID)
		require.NoError(t, err)
		require.Nil(t, p.RecipientID)
		history, err := hs.AdminGetAssignmentsHistory(device.ID)
		require.NoError(t, err)
		require.Len(t, history, 1)
		require.Zero(t, history[0].ModeratorID)
		redirect(admin, "/admin/halcones", url.Values{"name": {"Dispositivo administrativo"}, "moderator_id": {fmt.Sprint(mod.ID)}})
		var h models.Halcon
		require.NoError(t, facades.Orm().Query().Where("name = ?", "Dispositivo administrativo").FirstOrFail(&h))
		require.Equal(t, admin.ID, h.ModeratorID)
		for _, suffix := range []string{"", "/edit", "/assign"} {
			r, err := admin.request("GET", fmt.Sprintf("/admin/halcones/%d%s", h.ID, suffix), nil)
			expectResponse(t, r, err, 200)
		}
		redirect(admin, fmt.Sprintf("/admin/halcones/%d", h.ID), url.Values{"name": {"Actualizado por admin"}, "is_active": {"1"}})
		redirect(admin, fmt.Sprintf("/admin/halcones/%d/assign", h.ID), url.Values{"user_id": {fmt.Sprint(user.ID)}, "package_id": {"PKG-ADMIN"}})
		redirect(admin, fmt.Sprintf("/admin/halcones/%d/delete", h.ID), url.Values{})
		_, err = hs.GetByID(h.ID)
		require.Error(t, err)
		redirect(admin, fmt.Sprintf("/admin/users/%d/delete", admin.ID), url.Values{})
		_, err = us.GetByID(admin.ID)
		require.NoError(t, err)
	})
	t.Run("user_queries_and_search_role_and_account_revocation", func(t *testing.T) {
		all, total, err := us.GetAll(0, 101)
		require.NoError(t, err)
		require.Len(t, all, int(total))
		_, total, err = us.AdminListUsers(1, 20, "example.test", "user")
		require.NoError(t, err)
		require.Equal(t, int64(3), total)
		_, total, err = us.SearchAssignable(1, 20, strings.ToUpper(recipient.Email))
		require.NoError(t, err)
		require.Equal(t, int64(1), total)
		_, err = us.AdminCountByRole()
		require.NoError(t, err)
		ok, err := us.HasRole(admin.ID, "admin")
		require.NoError(t, err)
		require.True(t, ok)
		_, err = us.AdminGetUser(999999)
		require.Error(t, err)
		require.NoError(t, us.AdminUpdateUser(mod.ID, map[string]any{"role": "user"}))
		r, err := mod.request("GET", "/moderator/halcones", nil)
		expectResponse(t, r, err, 403)
		require.NoError(t, us.Update(user.ID, map[string]any{"status": false}))
		allowed, err := hs.CanView(user.PersonalID, user.ID)
		require.NoError(t, err)
		require.False(t, allowed)
		r, err = user.request("GET", "/api/tracking", nil)
		expectResponse(t, r, err, 401)
		require.Error(t, user.login())
		require.NoError(t, us.Update(user.ID, map[string]any{"status": true}))
		r, err = recipient.post("/api/auth/logout", url.Values{})
		expectResponse(t, r, err, 204)
		r, err = recipient.request("GET", "/api/session", nil)
		expectResponse(t, r, err, 200)
		var session map[string]any
		require.NoError(t, json.Unmarshal(r.body, &session))
		require.Nil(t, session["user"])
		r, err = user.post("/logout", url.Values{})
		expectRedirect(t, r, err)
	})
}
