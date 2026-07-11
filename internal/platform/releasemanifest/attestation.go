package releasemanifest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

const inTotoPayloadType = "application/vnd.in-toto+json"

type dsseEnvelope struct {
	PayloadType string            `json:"payloadType"`
	Payload     string            `json:"payload"`
	Signatures  []json.RawMessage `json:"signatures"`
}

type inTotoSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
}

type inTotoStatement struct {
	Type          string          `json:"_type"`
	Subject       []inTotoSubject `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

// VerifyAttestation correlates Cosign's already-verified DSSE output with the
// expected local predicate and immutable image digest.
func VerifyAttestation(verificationOutput, expectedPredicate []byte, imageRef string) error {
	expectedDigest, err := digestFromImageRef(imageRef)
	if err != nil {
		return err
	}
	expectedValue, err := decodeJSONValue(expectedPredicate)
	if err != nil {
		return fmt.Errorf("releasemanifest: expected predicate JSON is invalid: %w", err)
	}

	envelopes, err := decodeEnvelopes(verificationOutput)
	if err != nil {
		return fmt.Errorf("releasemanifest: verification output JSON is invalid: %w", err)
	}
	digestMatched := false
	predicateMatched := false
	for index, rawEnvelope := range envelopes {
		var envelope dsseEnvelope
		if err := decodeStrict(rawEnvelope, &envelope); err != nil {
			return fmt.Errorf("releasemanifest: verification envelope %d is malformed: %w", index, err)
		}
		if envelope.PayloadType != inTotoPayloadType || envelope.Payload == "" || len(envelope.Signatures) == 0 {
			return fmt.Errorf("releasemanifest: verification envelope %d is malformed", index)
		}
		payload, err := base64.StdEncoding.DecodeString(envelope.Payload)
		if err != nil {
			return fmt.Errorf("releasemanifest: verification envelope %d payload is not valid base64: %w", index, err)
		}
		var statement inTotoStatement
		if err := decodeStrict(payload, &statement); err != nil {
			return fmt.Errorf("releasemanifest: verification envelope %d statement is malformed: %w", index, err)
		}
		if statement.Type != "https://in-toto.io/Statement/v0.1" && statement.Type != "https://in-toto.io/Statement/v1" {
			return fmt.Errorf("releasemanifest: verification envelope %d statement type is invalid", index)
		}
		if statement.PredicateType == "" || len(statement.Predicate) == 0 || len(statement.Subject) == 0 {
			return fmt.Errorf("releasemanifest: verification envelope %d statement is malformed", index)
		}

		matchesDigest := false
		for _, subject := range statement.Subject {
			if subject.Digest["sha256"] == expectedDigest {
				matchesDigest = true
				break
			}
		}
		if !matchesDigest {
			continue
		}
		digestMatched = true
		predicateValue, err := decodeJSONValue(statement.Predicate)
		if err != nil {
			return fmt.Errorf("releasemanifest: verification envelope %d predicate is invalid: %w", index, err)
		}
		if reflect.DeepEqual(predicateValue, expectedValue) {
			predicateMatched = true
		}
	}
	if !digestMatched {
		return errors.New("releasemanifest: attestation subject digest mismatch")
	}
	if !predicateMatched {
		return errors.New("releasemanifest: attestation predicate mismatch")
	}
	return nil
}

func decodeEnvelopes(raw []byte) ([]json.RawMessage, error) {
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, errors.New("verification output is empty")
	}
	var envelopes []json.RawMessage
	switch trimmed[0] {
	case '{':
		envelopes = []json.RawMessage{json.RawMessage(bytes.Clone(trimmed))}
	case '[':
		if err := json.Unmarshal(trimmed, &envelopes); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("verification output must be a JSON object or array")
	}
	if len(envelopes) == 0 {
		return nil, errors.New("verification output has no envelopes")
	}
	return envelopes, nil
}

func digestFromImageRef(imageRef string) (string, error) {
	if strings.TrimSpace(imageRef) != imageRef || containsControl(imageRef) || strings.Count(imageRef, "@") != 1 {
		return "", errors.New("releasemanifest: image ref must contain one immutable digest")
	}
	name, digest, ok := strings.Cut(imageRef, "@")
	if !ok || name == "" || !digestPattern.MatchString(digest) {
		return "", errors.New("releasemanifest: image ref digest is invalid")
	}
	return strings.TrimPrefix(digest, "sha256:"), nil
}
