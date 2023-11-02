package common

import (
	"context"
	"database/sql"
	"sort"

	sq "github.com/Masterminds/squirrel"
	"go.flipt.io/flipt/internal/storage"
	flipt "go.flipt.io/flipt/rpc/flipt"
	"go.flipt.io/flipt/rpc/flipt/data"
)

func (s *Store) GetEvaluationRules(ctx context.Context, namespaceKey, flagKey string) (_ []*data.EvaluationRule, err error) {
	if namespaceKey == "" {
		namespaceKey = storage.DefaultNamespace
	}

	ruleMetaRows, err := s.builder.
		Select("id, \"rank\", segment_operator").
		From("rules").
		Where(sq.And{sq.Eq{"flag_key": flagKey}, sq.Eq{"namespace_key": namespaceKey}}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cerr := ruleMetaRows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	type RuleMeta struct {
		ID              string
		Rank            int32
		SegmentOperator flipt.SegmentOperator
	}

	var rmMap = make(map[string]*RuleMeta)

	ruleIDs := make([]string, 0)
	for ruleMetaRows.Next() {
		var rm RuleMeta

		if err := ruleMetaRows.Scan(&rm.ID, &rm.Rank, &rm.SegmentOperator); err != nil {
			return nil, err
		}

		rmMap[rm.ID] = &rm
		ruleIDs = append(ruleIDs, rm.ID)
	}

	if err := ruleMetaRows.Err(); err != nil {
		return nil, err
	}

	if err := ruleMetaRows.Close(); err != nil {
		return nil, err
	}

	rows, err := s.builder.Select(`
		rs.rule_id,
		rs.segment_key,
		s.match_type AS segment_match_type,
		c.id AS constraint_id,
		c."type" AS constraint_type,
		c.property AS constraint_property,
		c.operator AS constraint_operator,
		c.value AS constraint_value
	`).
		From("rule_segments AS rs").
		Join(`segments AS s ON rs.segment_key = s."key"`).
		LeftJoin(`constraints AS c ON (s."key" = c.segment_key AND s.namespace_key = c.namespace_key)`).
		Where(sq.Eq{"rs.rule_id": ruleIDs}).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var (
		uniqueRules    = make(map[string]*data.EvaluationRule)
		uniqueSegments = make(map[string]*data.EvaluationSegment)
		rules          = []*data.EvaluationRule{}
	)

	for rows.Next() {
		var (
			intermediateStorageRule struct {
				ID               string
				NamespaceKey     string
				FlagKey          string
				SegmentKey       string
				SegmentMatchType flipt.MatchType
				SegmentOperator  flipt.SegmentOperator
				Rank             int32
			}
			optionalConstraint optionalConstraint
		)

		if err := rows.Scan(
			&intermediateStorageRule.ID,
			&intermediateStorageRule.SegmentKey,
			&intermediateStorageRule.SegmentMatchType,
			&optionalConstraint.Id,
			&optionalConstraint.Type,
			&optionalConstraint.Property,
			&optionalConstraint.Operator,
			&optionalConstraint.Value); err != nil {
			return rules, err
		}

		rm := rmMap[intermediateStorageRule.ID]

		intermediateStorageRule.FlagKey = flagKey
		intermediateStorageRule.NamespaceKey = namespaceKey
		intermediateStorageRule.Rank = rm.Rank
		intermediateStorageRule.SegmentOperator = rm.SegmentOperator

		if _, ok := uniqueRules[intermediateStorageRule.ID]; ok {
			var c *data.EvaluationConstraint
			if optionalConstraint.Id.Valid {
				c = &data.EvaluationConstraint{
					Id:       optionalConstraint.Id.String,
					Type:     flipt.ComparisonType(optionalConstraint.Type.Int32),
					Property: optionalConstraint.Property.String,
					Operator: optionalConstraint.Operator.String,
					Value:    optionalConstraint.Value.String,
				}
			}

			segment, ok := uniqueSegments[intermediateStorageRule.SegmentKey]
			if !ok {
				ses := &data.EvaluationSegment{
					Key:       intermediateStorageRule.SegmentKey,
					MatchType: intermediateStorageRule.SegmentMatchType,
				}

				if c != nil {
					ses.Constraints = []*data.EvaluationConstraint{c}
				}

				uniqueSegments[intermediateStorageRule.SegmentKey] = ses
			} else if c != nil {
				segment.Constraints = append(segment.Constraints, c)
			}
		} else {
			// haven't seen this rule before
			newRule := &data.EvaluationRule{
				Id:              intermediateStorageRule.ID,
				Rank:            intermediateStorageRule.Rank,
				SegmentOperator: intermediateStorageRule.SegmentOperator,
			}

			var c *data.EvaluationConstraint
			if optionalConstraint.Id.Valid {
				c = &data.EvaluationConstraint{
					Id:       optionalConstraint.Id.String,
					Type:     flipt.ComparisonType(optionalConstraint.Type.Int32),
					Property: optionalConstraint.Property.String,
					Operator: optionalConstraint.Operator.String,
					Value:    optionalConstraint.Value.String,
				}
			}

			ses := &data.EvaluationSegment{
				Key:       intermediateStorageRule.SegmentKey,
				MatchType: intermediateStorageRule.SegmentMatchType,
			}

			if c != nil {
				ses.Constraints = []*data.EvaluationConstraint{c}
			}

			newRule.Segments = append(newRule.Segments, ses)

			uniqueRules[newRule.Id] = newRule
			rules = append(rules, newRule)
		}
	}

	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Rank < rules[j].Rank
	})

	if err := rows.Err(); err != nil {
		return rules, err
	}

	if err := rows.Close(); err != nil {
		return rules, err
	}

	return rules, nil
}

func (s *Store) GetEvaluationDistributions(ctx context.Context, ruleID string) (_ []*data.EvaluationDistribution, err error) {
	rows, err := s.builder.Select("d.id, d.rule_id, d.variant_id, d.rollout, v.\"key\", v.attachment").
		From("distributions d").
		Join("variants v ON (d.variant_id = v.id)").
		Where(sq.Eq{"d.rule_id": ruleID}).
		OrderBy("d.created_at ASC").
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var distributions []*data.EvaluationDistribution

	for rows.Next() {
		var (
			d          data.EvaluationDistribution
			attachment sql.NullString
		)

		if err := rows.Scan(
			&d.Id, &d.RuleId, &d.VariantId, &d.Rollout, &d.VariantKey, &attachment,
		); err != nil {
			return distributions, err
		}

		if attachment.Valid {
			attachmentString, err := compactJSONString(attachment.String)
			if err != nil {
				return distributions, err
			}
			d.VariantAttachment = attachmentString
		}

		distributions = append(distributions, &d)
	}

	if err := rows.Err(); err != nil {
		return distributions, err
	}

	if err := rows.Close(); err != nil {
		return distributions, err
	}

	return distributions, nil
}

func (s *Store) GetEvaluationRollouts(ctx context.Context, namespaceKey, flagKey string) (_ []*data.EvaluationRollout, err error) {
	if namespaceKey == "" {
		namespaceKey = storage.DefaultNamespace
	}

	rows, err := s.builder.Select(`
		r.id,
		r."type",
		r."rank",
		rt.percentage,
		rt.value,
		rss.segment_key,
		rss.rollout_segment_value,
		rss.segment_operator,
		rss.match_type,
		rss.constraint_type,
		rss.constraint_property,
		rss.constraint_operator,
		rss.constraint_value
	`).
		From("rollouts AS r").
		LeftJoin("rollout_thresholds AS rt ON (r.id = rt.rollout_id)").
		LeftJoin(`(
		SELECT
			rs.rollout_id,
			rsr.segment_key,
			s.match_type,
			rs.value AS rollout_segment_value,
			rs.segment_operator AS segment_operator,
			c."type" AS constraint_type,
			c.property AS constraint_property,
			c.operator AS constraint_operator,
			c.value AS constraint_value
		FROM rollout_segments AS rs
		JOIN rollout_segment_references AS rsr ON (rs.id = rsr.rollout_segment_id)
		JOIN segments AS s ON (rsr.segment_key = s."key")
		LEFT JOIN constraints AS c ON (rsr.segment_key = c.segment_key)
	) rss ON (r.id = rss.rollout_id)
	`).
		Where(sq.And{sq.Eq{"r.namespace_key": namespaceKey}, sq.Eq{"r.flag_key": flagKey}}).
		OrderBy(`r."rank" ASC`).
		QueryContext(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		if cerr := rows.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	var (
		uniqueSegmentedRollouts = make(map[string]*data.EvaluationRollout)
		uniqueSegments          = make(map[string]*data.EvaluationSegment)
		rollouts                = []*data.EvaluationRollout{}
	)

	for rows.Next() {
		var (
			rolloutId          string
			evaluationRollout  data.EvaluationRollout
			rtPercentageNumber sql.NullFloat64
			rtPercentageValue  sql.NullBool
			rsSegmentKey       sql.NullString
			rsSegmentValue     sql.NullBool
			rsSegmentOperator  sql.NullInt32
			rsMatchType        sql.NullInt32
			optionalConstraint optionalConstraint
		)

		if err := rows.Scan(
			&rolloutId,
			&evaluationRollout.Type,
			&evaluationRollout.Rank,
			&rtPercentageNumber,
			&rtPercentageValue,
			&rsSegmentKey,
			&rsSegmentValue,
			&rsSegmentOperator,
			&rsMatchType,
			&optionalConstraint.Type,
			&optionalConstraint.Property,
			&optionalConstraint.Operator,
			&optionalConstraint.Value,
		); err != nil {
			return rollouts, err
		}

		if rtPercentageNumber.Valid && rtPercentageValue.Valid {
			storageThreshold := &data.EvaluationRolloutThreshold{
				Percentage: float32(rtPercentageNumber.Float64),
				Value:      rtPercentageValue.Bool,
			}

			evaluationRollout.Rule = &data.EvaluationRollout_Threshold{
				Threshold: storageThreshold,
			}
		} else if rsSegmentKey.Valid &&
			rsSegmentValue.Valid &&
			rsSegmentOperator.Valid &&
			rsMatchType.Valid {

			var c *data.EvaluationConstraint
			if optionalConstraint.Type.Valid {
				c = &data.EvaluationConstraint{
					Type:     flipt.ComparisonType(optionalConstraint.Type.Int32),
					Property: optionalConstraint.Property.String,
					Operator: optionalConstraint.Operator.String,
					Value:    optionalConstraint.Value.String,
				}
			}

			if _, ok := uniqueSegmentedRollouts[rolloutId]; ok {
				// check if segment exists and either append constraints to an already existing segment,
				// or add another segment to the map.
				es, innerOk := uniqueSegments[rsSegmentKey.String]
				if innerOk {
					if c != nil {
						es.Constraints = append(es.Constraints, c)
					}
				} else {

					ses := &data.EvaluationSegment{
						Key:       rsSegmentKey.String,
						MatchType: flipt.MatchType(rsMatchType.Int32),
					}

					if c != nil {
						ses.Constraints = []*data.EvaluationConstraint{c}
					}

					uniqueSegments[rsSegmentKey.String] = ses
				}

				continue
			}

			storageSegment := &data.EvaluationRolloutSegment{
				Value:           rsSegmentValue.Bool,
				SegmentOperator: flipt.SegmentOperator(rsSegmentOperator.Int32),
			}

			ses := &data.EvaluationSegment{
				Key:       rsSegmentKey.String,
				MatchType: flipt.MatchType(rsMatchType.Int32),
			}

			if c != nil {
				ses.Constraints = []*data.EvaluationConstraint{c}
			}

			storageSegment.Segments = append(storageSegment.Segments, ses)

			evaluationRollout.Rule = &data.EvaluationRollout_Segment{
				Segment: storageSegment,
			}
			uniqueSegmentedRollouts[rolloutId] = &evaluationRollout
		}

		rollouts = append(rollouts, &evaluationRollout)
	}

	if err := rows.Err(); err != nil {
		return rollouts, err
	}

	return rollouts, nil
}
