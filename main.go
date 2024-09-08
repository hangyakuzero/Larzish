package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/calvinmclean/babyapi"
	"github.com/calvinmclean/babyapi/extensions"
	"github.com/calvinmclean/babyapi/html"

	"github.com/go-chi/render"
)

const (
	replicateURL string = "https://homepage.replicate.com/"
	model        string = "black-forest-labs/flux-schnell"
	version      string = "f2ab8a5bfe79f02f0789a146cf5e73d2a4ff2684a98c2b303d1e1ff3814271db"

	allResults         html.Template = "allResults"
	allResultsTemplate string        = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Results</title>
		<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/uikit@3.17.11/dist/css/uikit.min.css"/>
		<script src="https://unpkg.com/htmx.org@2.0.2"></script>
		<script src="https://unpkg.com/htmx-ext-sse@2.2.2/sse.js"></script>
	</head>

	<style>
		tr.htmx-swapping td {
			opacity: 0;
			transition: opacity 1s ease-out;
		}
		.prompt {
			font-style: italic;
		}
	</style>

	<body>
		<table class="uk-table uk-table-divider uk-margin-left uk-margin-right">
			<colgroup>
				<col>
				<col style="width: 300px;">
			</colgroup>

			<thead>
				<tr>
					<th>Prompt</th>
					<th></th>
				</tr>
			</thead>

			<tbody hx-ext="sse" sse-connect="/results/listen" sse-swap="newResult" hx-swap="afterend">
				<form hx-post="/results" hx-swap="none" hx-on::after-request="this.reset()">
					<td>
						<input class="uk-input" name="Prompt" type="text">
					</td>
					<td>
						<button type="submit" class="uk-button uk-button-primary">GENERATE</button>
					</td>
				</form>

				{{ range . }}
				{{ template "resultRow" . }}
				{{ end }}
			</tbody>
		</table>
	</body>
</html>`

	resultRow         html.Template = "resultRow"
	resultRowTemplate string        = `<tr hx-target="this" hx-swap="innerHTML">
	<td>
	{{ range .Images }}<a href="{{ . }}"><img src="{{ . }}" alt="{{ . }}" style="width:300px; height:auto; margin-right:10px;"/></a>{{ end }}
	<p class="prompt">{{ .Prompt }}</p>
	</td>
	<td>
		<button class="uk-button uk-button-danger" hx-delete="/results/{{ .ID }}" hx-swap="swap:1s">
			Delete
		</button>
	</td>
</tr>`
)

type PredictionResponse struct {
	ID string `json:"id"`
}

type PollResponse struct {
	Status string   `json:"status"`
	Output []string `json:"output"`
	Error  string   `json:"error,omitempty"`
}

type Result struct {
	babyapi.DefaultResource

	Prompt string
	Images []string
}

func (t *Result) HTML(_ http.ResponseWriter, r *http.Request) string {
	return resultRow.Render(r, t)
}

type AllResults struct {
	babyapi.ResourceList[*Result]
}

func (at AllResults) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

func (at AllResults) HTML(_ http.ResponseWriter, r *http.Request) string {
	return allResults.Render(r, at.Items)
}

func createAPI() *babyapi.API[*Result] {
	api := babyapi.NewAPI("API", "/results", func() *Result { return &Result{} })

	api.AddCustomRootRoute(http.MethodGet, "/", http.RedirectHandler("/results", http.StatusFound))

	api.SetGetAllResponseWrapper(func(results []*Result) render.Renderer {
		return AllResults{ResourceList: babyapi.ResourceList[*Result]{Items: results}}
	})

	api.ApplyExtension(extensions.HTMX[*Result]{})

	resultChan := api.AddServerSentEventHandler("/listen")

	api.SetOnCreateOrUpdate(func(w http.ResponseWriter, r *http.Request, t *Result) *babyapi.ErrResponse {
		if r.Method != http.MethodPost {
			return nil
		}

		t.Images = []string{}

		var wg sync.WaitGroup
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(prompt string, wg *sync.WaitGroup) error {
				defer wg.Done()
				pollID, err := getID(prompt)
				if err != nil {
					errb := new(babyapi.ErrResponse)
					errb.Err = err
					errb.ErrorText = err.Error()
					errb.HTTPStatusCode = http.StatusInternalServerError
					return errb
				}
				if pollID == "" {
					errb := new(babyapi.ErrResponse)
					errb.Err = errors.New("no ID found in response")
					errb.ErrorText = errb.Err.Error()
					errb.HTTPStatusCode = http.StatusInternalServerError
					return errb
				}
				image, err := getImageURL(pollID)
				if err != nil {
					errb := new(babyapi.ErrResponse)
					errb.Err = err
					errb.ErrorText = err.Error()
					errb.HTTPStatusCode = http.StatusInternalServerError
					return errb
				}
				t.Images = append(t.Images, image)
				return nil
			}(t.Prompt, &wg)
		}

		wg.Wait()

		select {
		case resultChan <- &babyapi.ServerSentEvent{Event: "newResult", Data: t.HTML(w, r)}:
		default:
			logger := babyapi.GetLoggerFromContext(r.Context())
			logger.Info("no listeners for server-sent event")
		}
		return nil
	})

	html.SetMap(map[string]string{
		string(allResults): allResultsTemplate,
		string(resultRow):  resultRowTemplate,
	})

	return api
}

func main() {
	api := createAPI()
	api.RunCLI()
}

func getID(prompt string) (string, error) {
	requestBody := fmt.Sprintf(`{
    "model": "`+model+`",
    "version": "`+version+`",
    "input": {
      "prompt": "%s"
    }
  }`, prompt)

	response, err := http.Post(replicateURL+"api/prediction", "application/json", bytes.NewBufferString(requestBody))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()

	var predictionData PredictionResponse
	content, _ := io.ReadAll(response.Body)
	if err := json.Unmarshal(content, &predictionData); err != nil {
		return "", err

	}

	return predictionData.ID, nil
}

func getImageURL(id string) (string, error) {
	timeout := time.NewTimer(60 * time.Second)
	ticker := time.NewTicker(5 * time.Second)
	defer timeout.Stop()
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return "", errors.New("timeout")
		case <-ticker.C:
			url := fmt.Sprintf("%s/api/poll?id=%s", replicateURL, id)
			response, err := http.Get(url)
			if err != nil {
				return "", err
			}
			defer response.Body.Close()

			var pollData PollResponse
			content, _ := io.ReadAll(response.Body)
			if err := json.Unmarshal(content, &pollData); err != nil {
				return "", err
			}

			switch pollData.Status {
			case "succeeded":
				if len(pollData.Output) > 0 {
					return pollData.Output[0], nil
				}
				return "", errors.New("output array is empty or not available")
			case "failed":
				return "", fmt.Errorf("prediction failed: %v", pollData.Error)
			default:
			}
		}
	}
}
