========================
PART 5B: MULTI-SHOT RULE (12s ONLY)
========================

- Scenes with duration 4s or 8s:
  MUST use a single camera configuration.
  One scene = one continuous shot.
  Multi-shot cutting inside the scene is NOT allowed.

- Scenes with duration 12s:
  MAY include multiple internal shots.
  If multiple shots are used:
    - The scene MUST define "camera_plan" instead of "camera".
    - camera_plan MUST be an array.
    - Each entry MUST include:
        - t_range
        - shot_type
        - angle
        - movement
        - movement_speed
        - focus
        - composition
    - camera_plan t_ranges MUST:
        - start at 0.0
        - cover full 12.0 seconds
        - not overlap
        - maintain 1 decimal precision.

- Even in 12s multi-shot scenes:
  - The final t_range MUST follow TAIL HOLD STABILITY RULE.
  - No internal cut is allowed inside the final safety_pad segment.



- Internal shot changes are NOT allowed inside the final safety_pad duration.