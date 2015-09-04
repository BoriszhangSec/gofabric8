package client

import (
	"fmt"

	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type SubjectAccessReviewsImpersonator interface {
	ImpersonateSubjectAccessReviews(token string) SubjectAccessReviewInterface
}

// SubjectAccessReviews has methods to work with SubjectAccessReview resources in the cluster scope
type SubjectAccessReviews interface {
	SubjectAccessReviews() SubjectAccessReviewInterface
}

// SubjectAccessReviewInterface exposes methods on SubjectAccessReview resources.
type SubjectAccessReviewInterface interface {
	Create(policy *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error)
}

// subjectAccessReviews implements SubjectAccessReviews interface
type subjectAccessReviews struct {
	r     *Client
	token string
}

// newImpersonatingSubjectAccessReviews returns a subjectAccessReviews
func newImpersonatingSubjectAccessReviews(c *Client, token string) *subjectAccessReviews {
	return &subjectAccessReviews{
		r:     c,
		token: token,
	}
}

// newSubjectAccessReviews returns a subjectAccessReviews
func newSubjectAccessReviews(c *Client) *subjectAccessReviews {
	return &subjectAccessReviews{
		r: c,
	}
}

func (c *subjectAccessReviews) Create(sar *authorizationapi.SubjectAccessReview) (result *authorizationapi.SubjectAccessReviewResponse, err error) {
	result = &authorizationapi.SubjectAccessReviewResponse{}

	// if this a cluster SAR, then no special handling
	if len(sar.Action.Namespace) == 0 {
		err = overrideAuth(c.token, c.r.Post().Resource("subjectAccessReviews")).Body(sar).Do().Into(result)
		return
	}

	err = c.r.Post().Resource("subjectAccessReviews").Body(sar).Do().Into(result)

	// if the namespace values don't match then we definitely hit an old server.  If we got a forbidden, then we might have hit an old server
	// and should try the old endpoint
	if (sar.Action.Namespace != result.Namespace) || kapierrors.IsForbidden(err) {
		deprecatedResponse := &authorizationapi.SubjectAccessReviewResponse{}
		deprecatedAttemptErr := overrideAuth(c.token, c.r.Post().Namespace(sar.Action.Namespace).Resource("subjectAccessReviews")).Body(sar).Do().Into(deprecatedResponse)

		// if we definitely hit an old server, then return the error and result you get from the older server.
		if sar.Action.Namespace != result.Namespace {
			return deprecatedResponse, deprecatedAttemptErr
		}

		// if we're not certain it was an old server, success overwrites the previous error, but failure doesn't overwrite the previous error
		if deprecatedAttemptErr == nil {
			err = nil
			result = deprecatedResponse
		}
	}

	return
}

func overrideAuth(token string, req *client.Request) *client.Request {
	if len(token) > 0 {
		req.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}
