package s3

import (
	"errors"
	"testing"

	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/roguekeyboards/mykeebs-api/internal/repository/s3/mocks"
)

type ECPartImageStoreSuite struct {
	suite.Suite

	mockClient  *mocks.MockS3API
	mockPresign *mocks.MockS3PresignAPI
	mockSigner  *mocks.MockGetSigner
	mockCDN     *mocks.MockInvalidator
	store       *ECPartImageStore
}

func TestECPartImageStoreSuite(t *testing.T) {
	suite.Run(t, new(ECPartImageStoreSuite))
}

func (s *ECPartImageStoreSuite) SetupTest() {
	s.mockClient = mocks.NewMockS3API(s.T())
	s.mockPresign = mocks.NewMockS3PresignAPI(s.T())
	s.mockSigner = mocks.NewMockGetSigner(s.T())
	s.mockCDN = mocks.NewMockInvalidator(s.T())
	s.store = &ECPartImageStore{
		client:  s.mockClient,
		presign: s.mockPresign,
		bucket:  "images-bucket",
		get:     s.mockSigner,
		cdn:     s.mockCDN,
	}
}

func (s *ECPartImageStoreSuite) TestSignGet_SignsKeyOnImagesDomain() {
	s.mockSigner.EXPECT().Sign("ec-parts/alice/sw1/image/up1").Return("https://images.example.com/signed", nil)

	url, err := s.store.SignGet(s.T().Context(), "ec-parts/alice/sw1/image/up1")

	s.Require().NoError(err)
	s.Equal("https://images.example.com/signed", url)
}

func (s *ECPartImageStoreSuite) TestSignGet_SignerError_Propagates() {
	s.mockSigner.EXPECT().Sign(mock.Anything).Return("", errors.New("crypto/rsa: verification error"))

	url, err := s.store.SignGet(s.T().Context(), "ec-parts/alice/sw1/image/up1")

	s.Require().ErrorContains(err, "crypto/rsa: verification error")
	s.Empty(url)
}

func (s *ECPartImageStoreSuite) TestPresignPut_Succeeds() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.MatchedBy(func(in *s3.PutObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "ec-parts/alice/sw1/image/up1" && *in.ContentType == "image/png" &&
				*in.ContentLength == 1234 && *in.IfNoneMatch == "*"
		})).
		Return(&v4.PresignedHTTPRequest{URL: "https://example.com/presigned-put"}, nil)

	url, err := s.store.PresignPut(s.T().Context(), "ec-parts/alice/sw1/image/up1", "image/png", 1234)

	s.Require().NoError(err)
	s.Equal("https://example.com/presigned-put", url)
}

func (s *ECPartImageStoreSuite) TestPresignPut_SDKError_Propagates() {
	s.mockPresign.EXPECT().
		PresignPutObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	url, err := s.store.PresignPut(s.T().Context(), "ec-parts/alice/sw1/image/up1", "image/png", 1234)

	s.Require().ErrorContains(err, "s3: access denied")
	s.Empty(url)
}

func (s *ECPartImageStoreSuite) TestDelete_Succeeds() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.MatchedBy(func(in *s3.DeleteObjectInput) bool {
			return *in.Bucket == "images-bucket" && *in.Key == "ec-parts/alice/sw1/image/up1"
		})).
		Return(&s3.DeleteObjectOutput{}, nil)

	s.mockCDN.EXPECT().Invalidate(mock.Anything, []string{"/ec-parts/alice/sw1/image/up1"}).Return(nil)

	err := s.store.Delete(s.T().Context(), "ec-parts/alice/sw1/image/up1")

	s.Require().NoError(err)
}

func (s *ECPartImageStoreSuite) TestDelete_InvalidationFails_StillSucceeds() {
	s.mockClient.EXPECT().DeleteObject(mock.Anything, mock.Anything).Return(&s3.DeleteObjectOutput{}, nil)
	s.mockCDN.EXPECT().Invalidate(mock.Anything, mock.Anything).Return(errors.New("cloudfront: throttled"))

	err := s.store.Delete(s.T().Context(), "ec-parts/alice/sw1/image/up1")

	s.Require().NoError(err)
}

func (s *ECPartImageStoreSuite) TestDelete_SDKError_Propagates() {
	s.mockClient.EXPECT().
		DeleteObject(mock.Anything, mock.Anything).
		Return(nil, errors.New("s3: access denied"))

	err := s.store.Delete(s.T().Context(), "ec-parts/alice/sw1/image/up1")

	s.Require().ErrorContains(err, "s3: access denied")
}
