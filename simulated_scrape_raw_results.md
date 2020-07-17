# ingester, 20m data collection (slow-down 10x)

## resolution=20

Double:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 2076 (38.27%)
  5 bits → 2062 (38.02%)
  10 bits → 1062 (19.58%)
  14 bits → 224 (4.13%)
  TOTAL storage size for ΔΔ(Δ) values: 4265 bytes (53.3 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 2076 (38.27%)
  4 bits → 1715 (31.62%)
  7 bits → 800 (14.75%)
  10 bits → 609 (11.23%)
  14 bits → 224 (4.13%)
  TOTAL storage size for ΔΔ(Δ) values: 4115 bytes (51.4 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 1984 (38.25%)
  3 bits → 1202 (23.17%)
  6 bits → 1016 (19.59%)
  9 bits → 548 (10.56%)
  12 bits → 419 (8.08%)
  64 bits → 18 (0.35%)
  TOTAL storage size for ΔΔ(Δ) values: 4078 bytes (53.0 bytes per scrape)

Triple:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 1847 (33.96%)
  4 bits → 1926 (35.41%)
  8 bits → 1234 (22.69%)
  12 bits → 432 (7.94%)
  TOTAL storage size for ΔΔ(Δ) values: 4182 bytes (52.3 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 1847 (33.96%)
  4 bits → 1926 (35.41%)
  7 bits → 974 (17.91%)
  9 bits → 510 (9.38%)
  12 bits → 182 (3.35%)
  TOTAL storage size for ΔΔ(Δ) values: 4085 bytes (51.1 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 1729 (34.33%)
  3 bits → 1283 (25.48%)
  6 bits → 1152 (22.88%)
  9 bits → 706 (14.02%)
  12 bits → 166 (3.30%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 3814 bytes (50.9 bytes per scrape)

## resolution=100

Double:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 10671 (44.28%)
  4 bits → 8338 (34.60%)
  8 bits → 4169 (17.30%)
  12 bits → 919 (3.81%)
  TOTAL storage size for ΔΔ(Δ) values: 15042 bytes (188.0 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 10671 (44.28%)
  3 bits → 6566 (27.25%)
  6 bits → 3884 (16.12%)
  8 bits → 2057 (8.54%)
  12 bits → 919 (3.81%)
  TOTAL storage size for ΔΔ(Δ) values: 14730 bytes (184.1 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 10815 (44.32%)
  3 bits → 6615 (27.11%)
  6 bits → 4006 (16.42%)
  9 bits → 2721 (11.15%)
  12 bits → 244 (1.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 14933 bytes (184.4 bytes per scrape)

Triple:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 9555 (39.74%)
  3 bits → 7292 (30.33%)
  6 bits → 5532 (23.01%)
  9 bits → 1665 (6.92%)
  TOTAL storage size for ΔΔ(Δ) values: 14472 bytes (180.9 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 9555 (39.74%)
  3 bits → 7292 (30.33%)
  5 bits → 4078 (16.96%)
  7 bits → 2635 (10.96%)
  9 bits → 484 (2.01%)
  TOTAL storage size for ΔΔ(Δ) values: 14239 bytes (178.0 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 9672 (39.64%)
  3 bits → 7471 (30.62%)
  6 bits → 5569 (22.83%)
  9 bits → 1686 (6.91%)
  12 bits → 0 (0.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 14883 bytes (183.7 bytes per scrape)

# querier, 1h data collection

## resolution=20

Double:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 9113 (36.18%)
  4 bits → 9209 (36.56%)
  6 bits → 4607 (18.29%)
  9 bits → 2260 (8.97%)
  TOTAL storage size for ΔΔ(Δ) values: 16618 bytes (69.2 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 9113 (36.18%)
  4 bits → 9209 (36.56%)
  6 bits → 4607 (18.29%)
  7 bits → 1422 (5.65%)
  9 bits → 838 (3.33%)
  TOTAL storage size for ΔΔ(Δ) values: 16545 bytes (68.9 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 8904 (35.99%)
  3 bits → 6452 (26.08%)
  6 bits → 6520 (26.36%)
  9 bits → 2863 (11.57%)
  12 bits → 0 (0.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 17132 bytes (72.6 bytes per scrape)

Triple:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 7939 (31.66%)
  3 bits → 8187 (32.65%)
  5 bits → 6062 (24.17%)
  8 bits → 2889 (11.52%)
  TOTAL storage size for ΔΔ(Δ) values: 16143 bytes (67.5 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 7939 (31.66%)
  3 bits → 8187 (32.65%)
  5 bits → 6062 (24.17%)
  6 bits → 2077 (8.28%)
  8 bits → 812 (3.24%)
  TOTAL storage size for ΔΔ(Δ) values: 15985 bytes (66.9 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 7896 (31.49%)
  3 bits → 8096 (32.29%)
  6 bits → 8243 (32.88%)
  9 bits → 836 (3.33%)
  12 bits → 0 (0.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 16678 bytes (69.8 bytes per scrape)

## resolution=100

Double:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 55770 (47.39%)
  3 bits → 38765 (32.94%)
  5 bits → 19996 (16.99%)
  7 bits → 3140 (2.67%)
  TOTAL storage size for ΔΔ(Δ) values: 55120 bytes (228.7 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 55770 (47.39%)
  3 bits → 38765 (32.94%)
  4 bits → 12475 (10.60%)
  5 bits → 7521 (6.39%)
  7 bits → 3140 (2.67%)
  TOTAL storage size for ΔΔ(Δ) values: 54893 bytes (227.8 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 54969 (47.36%)
  3 bits → 38375 (33.06%)
  6 bits → 22516 (19.40%)
  9 bits → 200 (0.17%)
  12 bits → 0 (0.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 56511 bytes (237.4 bytes per scrape)

Triple:
- Bit bucket frequency (4 buckets incl. zero bucket):
  0 bits → 49728 (42.44%)
  3 bits → 42026 (35.87%)
  5 bits → 23213 (19.81%)
  7 bits → 2193 (1.87%)
  TOTAL storage size for ΔΔ(Δ) values: 58436 bytes (243.5 bytes per scrape)
- Bit bucket frequency (5 buckets incl. zero bucket):
  0 bits → 49728 (42.44%)
  3 bits → 42026 (35.87%)
  4 bits → 14568 (12.43%)
  5 bits → 8645 (7.38%)
  7 bits → 2193 (1.87%)
  TOTAL storage size for ΔΔ(Δ) values: 57970 bytes (241.5 bytes per scrape)
- Bit bucket frequency (FIXED bucket schema):
  0 bits → 50469 (42.70%)
  3 bits → 42352 (35.83%)
  6 bits → 25306 (21.41%)
  9 bits → 63 (0.05%)
  12 bits → 0 (0.00%)
  64 bits → 0 (0.00%)
  TOTAL storage size for ΔΔ(Δ) values: 61350 bytes (253.5 bytes per scrape)
