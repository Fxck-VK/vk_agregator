package accountoauth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"vk-ai-aggregator/internal/domain"
)

// yandexProfile exchanges a server-only OAuth token for the stable subject.
// Email, login and display metadata never establish identity ownership.
func (b *Browser) yandexProfile(ctx context.Context, clientID, token string) (domain.VerifiedAccountLogin, error) {
	empty := domain.VerifiedAccountLogin{}
	if len(token) == 0 || len(token) > 8192 {
		return empty, ErrInvalidAssertion
	}
	for _, r := range token {
		if r < 33 || r > 126 {
			return empty, ErrInvalidAssertion
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://login.yandex.ru/info?format=json", nil)
	if err != nil {
		return empty, ErrInvalidAssertion
	}
	req.Header.Set("Authorization", "OAuth "+token)
	req.Header.Set("Accept", "application/json")
	response, err := b.client.Do(req)
	if err != nil {
		return empty, ErrInvalidAssertion
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return empty, ErrInvalidAssertion
	}
	var info struct {
		ID        string `json:"id"`
		ClientID  string `json:"client_id"`
		Error     string `json:"error"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		RealName  string `json:"real_name"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(&info); err != nil || info.Error != "" || info.ClientID != clientID {
		return empty, ErrInvalidAssertion
	}
	normalized, err := domain.NormalizeExternalIdentity(domain.IdentityProviderYandex, info.ID)
	if err != nil || normalized != info.ID {
		return empty, ErrInvalidAssertion
	}
	name := domain.NormalizeAccountDisplayName(info.FirstName + " " + info.LastName)
	if name == "" {
		name = domain.NormalizeAccountDisplayName(info.RealName)
	}
	return domain.VerifiedAccountLogin{Method: domain.AccountLoginYandex, ExternalID: info.ID, Verified: true, DisplayName: name}, nil
}
