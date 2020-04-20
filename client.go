package avs

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
)

// Multipart object returned by AVS.
type responsePart struct {
	Directive *Message
}

// Client enables making requests and creating downchannels to AVS.
type Client struct {
	EndpointURL string
}

// CreateDownchannel establishes a persistent connection with AVS and returns a
// read-only channel through which AVS will deliver directives.
func (c *Client) CreateDownchannel(accessToken string) (<-chan *Message, error) {
	req, err := http.NewRequest("GET", c.EndpointURL + DirectivesPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_4) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/80.0.3987.163 Safari/537.36")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if more, err := checkStatusCode(resp); !more {
		resp.Body.Close()
		return nil, err
	}
	directives := make(chan *Message)
	go func() {
		defer close(directives)
		defer resp.Body.Close()
		mr, err := newMultipartReaderFromResponse(resp)
		if err != nil {
			return
		}
		// TODO: Consider reporting errors.
		for {
			p, err := mr.NextPart()
			if err != nil {
				break
			}
			data, err := ioutil.ReadAll(p)
			if err != nil {
				break
			}
			var response responsePart
			err = json.Unmarshal(data, &response)
			if err != nil {
				break
			}
			directives <- response.Directive
		}
	}()
	return directives, nil
}

// Do posts a request to the AVS service's /events endpoint.
func (c *Client) Do(request *Request) (*Response, error) {
	body, bodyIn := io.Pipe()
	writer := multipart.NewWriter(bodyIn)
	go func() {
		// Write to pipe must be parallel to allow HTTP request to read
		err := writeJSON(writer, "metadata", request)
		if err != nil {
			bodyIn.CloseWithError(err)
			return
		}
		if request.Audio != nil {
			p, err := writer.CreateFormFile("audio", "audio.wav")
			if err != nil {
				bodyIn.CloseWithError(err)
				return
			}
			// Run io.Copy in goroutine so audio can be streamed
			_, err = io.Copy(p, request.Audio)
			if err != nil {
				bodyIn.CloseWithError(err)
				return
			}
		}
		err = writer.Close()
		if err != nil {
			bodyIn.CloseWithError(err)
			return
		}
		bodyIn.Close()
	}()
	// Send the request to AVS.
	req, err := http.NewRequest("POST", c.EndpointURL + EventsPath, body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", request.AccessToken))
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("Cookie", `nhnuserlocale=ko_KR; nhntimezone=Asia%2FSeoul%3A%2B9; language=ko_KR; LC=ko_KR; timezone=Asia/Seoul; TZ=Asia/Seoul; WORKS_RE_LOC=kr1; WORKS_TE_LOC=kr1; id_login_rememberId=check; UserLocale=ko-kr; country=KR; WORKS_USER_ID="jaehyuk2.shin@worksmobile.com"; nclouduserlocale=ko_KR; org.springframework.web.servlet.i18n.CookieLocaleResolver.LOCALE=ko_KR; _gid=GA1.2.1445329779.1587058511; _ga=GA1.1.1329235009.1586153643; _ga_N90K8EJMQ3=GS1.1.1587058511.8.1.1587058564.0; nssTok=0KR8zVuaFn5pO6WoSRIdLhTpU8Uqo0IxgDEruSyG13fGmYHcX25V_KvVBviltO_ANJP58oqgrg6gkbFLB_0IlO732UI1IvqQSC3__BEMcGtCAo2H5DwwayAyCv7u1QoK; neoidTempSessionKey=39E8GHBE; NMUSER=HqnraAEwaq2sKqtZKxgwaAgmKxg9KqvsaZdwaqgsaqRiaqtdKA29FonDKqgs6xRaaZnlad/sxoRaad/sa9vsaqROW9e7EoRaad/sawlObou516lvpB25W49vpSl5MBp0b4FTbNg5MreR7A2lKAgs1XUw7NF0Mre5pzJZDL9GW430DVl5W63474lC+4kZMreZbVloWrdnaAvmKAnDKxg5Kxu/7oumKL/lKAnwaxtq7AulFAtnKxvdFxnwKV/wKxU5KoKZ7oEma49GW6kjW6JGWJe5MXKs; NEO_SES="AAAA7rUdzqBjQ1bRDTH0CsCy6vgxRXreIXwd1foPW+0KZxyx7VUOizt3R+1HAalYSlRpZtRuwIOXRKvV/5OmF264wNVOOwjlQu781wbSy7HGxcYWwpy6o57P5JKZuHGE/Vyk6E/ybNi9qjS4Ie9NPcqaIUGGW+2+3uY+WMH2Pj432mBraOmnSTsm4LEJNaia+PxsjsOb+N9qlTy4N9r59nuiEYnCJmQWITvI18QKivW4LtGQDYtlq8dNmER0ILbrz3kkI2X+cs+RUQZ6ZmNWcluqWdpQ9HrbBXvep8cv9+fcwKU9zh7w84EEHjWaTF7pKt2PPQ=="; WORKS_SES=2D8FCA0E3D6214C28E92127694E1F0FF826CB7F22FEB59AEC2EF707B06C5019AD6773F601BEFC9DE55ADBC71DB951D15DB2BC524AB33A7C5B34B4FAEFF01B7DB58442AA66A59267D44C40EF23F549312334A7BC1DA6FFC9DBDC577E32845A451B14289C1557DCD7011AD97AC29260F13ABCEAF35FA328045329650B9058E2B31E1CC368B4D8AF03EA2689E948F77AAD6A881B87B80044795DE5D2DB44064BE9EEEECF49F29DF36842EB3ECF2808A0A5993E7C82BF20C4115C796A502BAB2C4AA170DAAF30131D65AB8813046487007AE5CA541554E066DC26651B381F6125AFF631DF6F6CF27E8C2CC58E46D110B94226409306DA1301957CCE0C18D7A4E4FE4BC495D9883ED4BB5F1E4BC7D1F7F83FBCB36A782469CED469C8D180C55C5F51633BDA03221780CB02BAAFDE09F2B518BB082D006C12A533E0B2AB4FC0EB1AB5A3C9E27E505DEC32300BBC648BC186E8611DCC66A85311CA1C572F96DE43D4E2225C29A42D5B05C6781B6B476FE51E510B13B2306AF5B0F234F766DD2CBC79568B27F365308FC2F2D6253AB620859F81C5A5CCB2551A3E9856E8E33EB43210CF9E17485959B02B450052E2B369EF2F3427D5ED3975EB6A8F9E2493A5D95DD8E7D66F4D75070A776FD817B7700D652F37D6A9812AE3894AA4EC626362D3633534AB14B7486870B7E6C6C75ACADC53A3F4EB9C6B3E043E9DD767FE6E1FBEA8A1DECC68F33E6EAE82328DEA6636596EACAB66DA8CFDB4C89CC1B3F5733829351B081AC9A344391D3950C464C8DE5D65D23A42EF0EFC6D1BD01DA57A854A57CA4E92A73974F882D475CD7B943ACA232A2CBC4420A5F746CE2CCD4B034F81495B777DA03EC1A46121108FCFC0FBC023FFF29057DD48A5F18ED7A56C50EB6905EAF11AF85F12D5B1E00C5054447B4DDAA6E0D5D7EB5822E120709A606CD85F88658A8C40F76C2FB7C6E098377F3DA49D1427A5FAB5F33DB1EE4CB93934CB97D9F9C5256B5F3C1228E372B81A2BD4770E572261E31805E50104196CCA3ACB0F6072DC69FA92C31F6B8905CB1D53FF69A7140E046084150C80316E0DB553A236111F9D28D; NEO_CHK="AAAAW7aVy8w71r50/HXwaYYfs+QAfuNP4G3gvWvS2yRbxpVIC2+gUr+YHXZU5UQCBYueGnrJIxYIdrC6Sz7VVkxOhO/L49gfI8XQRhqnd68uIN2FNQum8u7Hyk3GbAak6sNrPQ=="`)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	more, err := checkStatusCode(resp)
	if err != nil {
		return nil, err
	}
	response := &Response{
		RequestId:  resp.Header.Get("x-amzn-requestid"),
		Directives: []*Message{},
		Content:    map[string][]byte{},
	}
	if !more {
		// AVS returned an empty response, so there's nothing to parse.
		return response, nil
	}
	// Parse the multipart response.
	mr, err := newMultipartReaderFromResponse(resp)
	if err != nil {
		return nil, err
	}
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		mediatype, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
		if err != nil {
			return nil, err
		}
		data, err := ioutil.ReadAll(p)
		if err != nil {
			return nil, err
		}
		if contentId := p.Header.Get("Content-ID"); contentId != "" {
			// This part is a referencable piece of content.
			// XXX: Content-ID generally always has angle brackets, but there may be corner cases?
			response.Content[contentId[1:len(contentId)-1]] = data
		} else if mediatype == "application/json" {
			// This is a directive.
			var resp responsePart
			err = json.Unmarshal(data, &resp)
			if err != nil {
				return nil, err
			}
			if resp.Directive == nil {
				return nil, fmt.Errorf("missing directive %s", string(data))
			}
			response.Directives = append(response.Directives, resp.Directive)
		} else {
			return nil, fmt.Errorf("unhandled part %s", p)
		}
	}
	return response, nil
}

// Ping will ping AVS on behalf of a user to indicate that the connection is
// still alive.
func (c *Client) Ping(accessToken string) error {
	// TODO: Once Go supports sending PING frames, that would be a better alternative.
	req, err := http.NewRequest("GET", c.EndpointURL + PingPath, nil)
	if err != nil {
		return err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, err = checkStatusCode(resp)
	return err
}

// Checks the status code of the response and returns whether the caller should
// expect there to be more content, as well as any error.
//
// This function should only be called before the body has been read.
func checkStatusCode(resp *http.Response) (more bool, err error) {
	switch resp.StatusCode {
	case 200:
		// Keep going.
		return true, nil
	case 204:
		// No content.
		return false, nil
	default:
		// Attempt to parse the response as a System.Exception message.
		data, _ := ioutil.ReadAll(resp.Body)
		var exception Exception
		json.Unmarshal(data, &exception)
		if exception.Payload.Code != "" {
			return false, &exception
		}
		// Fallback error.
		return false, fmt.Errorf("request failed with %s", resp.Status)
	}
}
