# 💰 Google Cloud Run - Cost Estimation

## Configuration de l'instance

### Ressources allouées
- **CPU**: 1 vCPU
- **Mémoire**: 512 MiB (0.5 GiB)
- **Région**: europe-west1 (Belgique) - prix standard
- **Concurrency**: 80 requêtes simultanées par instance
- **Min instances**: 0 (scale-to-zero activé)
- **Max instances**: 10

### Tarifs Google Cloud Run (Novembre 2024)

#### Free Tier (par mois)
- ✅ 2 million de requêtes
- ✅ 360,000 vCPU-secondes
- ✅ 180,000 GiB-secondes de mémoire
- ✅ 1 GiB de réseau egress (vers Amérique du Nord)

#### Tarifs au-delà du Free Tier
- **CPU**: $0.00002400 par vCPU-seconde
- **Mémoire**: $0.00000250 par GiB-seconde
- **Requêtes**: $0.40 par million de requêtes
- **Réseau (egress)**: $0.12 par GiB (vers worldwide, hors Chine/Australie)

---

## 📊 Scénarios d'utilisation

### Scénario 1: Développement / Test (Très faible usage)

**Volume:**
- 1,000 requêtes/jour = 30,000 req/mois
- 50% cache HIT (~50ms), 50% cache MISS (~3s)
- Temps moyen par requête: ~1.5s

**Calcul des ressources:**

**Requêtes:**
- Total: 30,000 requêtes/mois
- **Coût**: GRATUIT (< 2M free tier)

**CPU (1 vCPU):**
- Cache HIT (15,000 req × 0.05s): 750 vCPU-s
- Cache MISS (15,000 req × 3s): 45,000 vCPU-s
- **Total**: 45,750 vCPU-s
- **Coût**: GRATUIT (< 360,000 free tier)

**Mémoire (0.5 GiB):**
- Cache HIT: 15,000 × 0.05s × 0.5 GiB = 375 GiB-s
- Cache MISS: 15,000 × 3s × 0.5 GiB = 22,500 GiB-s
- **Total**: 22,875 GiB-s
- **Coût**: GRATUIT (< 180,000 free tier)

**Réseau:**
- ~2 KB par réponse leaderboard
- 30,000 × 2 KB = 60 MB = 0.06 GiB
- **Coût**: GRATUIT (< 1 GiB free tier)

#### 💰 **Coût total mensuel: $0.00** (100% Free Tier)

---

### Scénario 2: Production Légère (Usage modéré)

**Volume:**
- 50,000 requêtes/jour = 1,500,000 req/mois
- 80% cache HIT (~50ms), 20% cache MISS (~3s)
- Traffic pattern: 8h-22h (14h actives par jour)

**Calcul des ressources:**

**Requêtes:**
- Total: 1,500,000 requêtes/mois
- **Coût**: GRATUIT (< 2M free tier)

**CPU (1 vCPU):**
- Cache HIT (1,200,000 req × 0.05s): 60,000 vCPU-s
- Cache MISS (300,000 req × 3s): 900,000 vCPU-s
- **Total**: 960,000 vCPU-s
- Free Tier: 360,000 vCPU-s
- **Billable**: 600,000 vCPU-s × $0.000024 = **$14.40**

**Mémoire (0.5 GiB):**
- Cache HIT: 1,200,000 × 0.05s × 0.5 = 30,000 GiB-s
- Cache MISS: 300,000 × 3s × 0.5 = 450,000 GiB-s
- **Total**: 480,000 GiB-s
- Free Tier: 180,000 GiB-s
- **Billable**: 300,000 GiB-s × $0.0000025 = **$0.75**

**Réseau:**
- 1,500,000 × 2 KB = 3 GB = 3 GiB
- Free Tier: 1 GiB
- **Billable**: 2 GiB × $0.12 = **$0.24**

**Requêtes supplémentaires:** $0.00 (dans free tier)

#### 💰 **Coût total mensuel: $15.39**
#### 💰 **Coût par requête: $0.00001026** (0.001 centime)

---

### Scénario 3: Production Moyenne (Usage élevé)

**Volume:**
- 200,000 requêtes/jour = 6,000,000 req/mois
- 90% cache HIT (~50ms), 10% cache MISS (~3s)
- Traffic pattern: 24/7 avec pics

**Calcul des ressources:**

**Requêtes:**
- Total: 6,000,000 requêtes/mois
- Free Tier: 2,000,000
- **Billable**: 4,000,000 req × ($0.40/1M) = **$1.60**

**CPU (1 vCPU):**
- Cache HIT (5,400,000 req × 0.05s): 270,000 vCPU-s
- Cache MISS (600,000 req × 3s): 1,800,000 vCPU-s
- **Total**: 2,070,000 vCPU-s
- Free Tier: 360,000 vCPU-s
- **Billable**: 1,710,000 vCPU-s × $0.000024 = **$41.04**

**Mémoire (0.5 GiB):**
- Cache HIT: 5,400,000 × 0.05s × 0.5 = 135,000 GiB-s
- Cache MISS: 600,000 × 3s × 0.5 = 900,000 GiB-s
- **Total**: 1,035,000 GiB-s
- Free Tier: 180,000 GiB-s
- **Billable**: 855,000 GiB-s × $0.0000025 = **$2.14**

**Réseau:**
- 6,000,000 × 2 KB = 12 GB = 12 GiB
- Free Tier: 1 GiB
- **Billable**: 11 GiB × $0.12 = **$1.32**

#### 💰 **Coût total mensuel: $46.10**
#### 💰 **Coût par requête: $0.000007683** (0.0008 centime)

---

### Scénario 4: Production Intensive (Usage très élevé)

**Volume:**
- 1,000,000 requêtes/jour = 30,000,000 req/mois
- 95% cache HIT (~50ms), 5% cache MISS (~3s)
- Traffic pattern: 24/7 avec haute disponibilité

**Calcul des ressources:**

**Requêtes:**
- Total: 30,000,000 requêtes/mois
- Free Tier: 2,000,000
- **Billable**: 28,000,000 req × ($0.40/1M) = **$11.20**

**CPU (1 vCPU):**
- Cache HIT (28,500,000 req × 0.05s): 1,425,000 vCPU-s
- Cache MISS (1,500,000 req × 3s): 4,500,000 vCPU-s
- **Total**: 5,925,000 vCPU-s
- Free Tier: 360,000 vCPU-s
- **Billable**: 5,565,000 vCPU-s × $0.000024 = **$133.56**

**Mémoire (0.5 GiB):**
- Cache HIT: 28,500,000 × 0.05s × 0.5 = 712,500 GiB-s
- Cache MISS: 1,500,000 × 3s × 0.5 = 2,250,000 GiB-s
- **Total**: 2,962,500 GiB-s
- Free Tier: 180,000 GiB-s
- **Billable**: 2,782,500 GiB-s × $0.0000025 = **$6.96**

**Réseau:**
- 30,000,000 × 2 KB = 60 GB = 60 GiB
- Free Tier: 1 GiB
- **Billable**: 59 GiB × $0.12 = **$7.08**

#### 💰 **Coût total mensuel: $158.80**
#### 💰 **Coût par requête: $0.000005293** (0.0005 centime)

---

## 📈 Tableau récapitulatif

| Scénario | Requêtes/mois | Cache HIT % | Coût/mois | Coût/req | Économie vs Always-On |
|----------|---------------|-------------|-----------|----------|----------------------|
| **Dev/Test** | 30K | 50% | **$0.00** | $0.000000 | 100% |
| **Prod Légère** | 1.5M | 80% | **$15.39** | $0.000010 | ~85% |
| **Prod Moyenne** | 6M | 90% | **$46.10** | $0.000008 | ~80% |
| **Prod Intensive** | 30M | 95% | **$158.80** | $0.000005 | ~75% |

---

## 🎯 Optimisations possibles

### 1. Augmenter le cache TTL (60s → 120s)
- **Impact**: -30% de cache MISS
- **Économie**: ~20-25% sur le coût total
- **Trade-off**: Données légèrement moins fraîches

### 2. Réduire la mémoire (512 MiB → 256 MiB)
- **Impact**: Service reste fonctionnel (leaderboard ~200-250 MB)
- **Économie**: -50% sur coût mémoire (~10-15% total)
- **Risque**: OOM si données dépassent 256 MB

### 3. CPU auto-scaling agressif
```yaml
autoscaling:
  minInstances: 0
  maxInstances: 5  # Au lieu de 10
  targetConcurrency: 100  # Au lieu de 80
```
- **Économie**: ~10% sur période faible charge

### 4. Utiliser min_instances: 1 pendant pics
- **Coût additionnel**: +$8-12/mois
- **Bénéfice**: Cold start éliminé (pas de latence ~2-3s)
- **Recommandé pour**: Production intensive uniquement

---

## 🔍 Comparaison avec d'autres solutions

### Cloud Run vs Compute Engine (e2-micro)

| Ressource | Cloud Run (Scénario 3) | e2-micro Always-On | Économie |
|-----------|------------------------|-------------------|----------|
| **CPU** | 1 vCPU (on-demand) | 0.25-1 vCPU shared | - |
| **Mémoire** | 512 MiB | 1 GB | - |
| **Coût mensuel** | **$46.10** | **$7.11** (preemptible) ou **$24.27** (standard) | Variable |
| **Disponibilité** | Auto-scaling | Manuel | Cloud Run wins |
| **Maintenance** | Zéro | Patching, updates | Cloud Run wins |

**Verdict**: 
- **< 1M req/mois**: Cloud Run moins cher (free tier)
- **1-5M req/mois**: Cloud Run compétitif avec scale-to-zero
- **> 10M req/mois**: e2-small/medium plus économique si 24/7

### Cloud Run vs Cloud Functions

| Ressource | Cloud Run | Cloud Functions (2nd gen) |
|-----------|-----------|--------------------------|
| **Free Tier** | ✅ Généreux | ✅ Similaire |
| **Cold Start** | ~1-2s | ~2-3s |
| **Max duration** | 60 min | 60 min |
| **Coût (6M req)** | **$46.10** | ~$52-58 |

**Verdict**: Cloud Run est 10-15% moins cher et plus flexible

---

## 💡 Recommandations

### Pour votre cas (Hypermetrics)

#### Configuration optimale recommandée:
```yaml
resources:
  limits:
    cpu: "1"
    memory: 512Mi
  
autoscaling:
  minInstances: 0  # Scale-to-zero activé
  maxInstances: 10
  targetConcurrency: 80

execution:
  timeout: 60s
  
containerConcurrency: 80
```

#### Budget prévisionnel par phase:

**Phase 1 - Lancement (0-3 mois):**
- Volume estimé: 50-100K req/mois
- **Budget**: $0-5/mois (free tier)

**Phase 2 - Croissance (3-12 mois):**
- Volume estimé: 500K-2M req/mois
- **Budget**: $10-25/mois

**Phase 3 - Production (12+ mois):**
- Volume estimé: 5-10M req/mois
- **Budget**: $40-80/mois

---

## 🚨 Alertes recommandées

### Cloud Monitoring - Budget Alerts

```yaml
budget:
  amount: 50  # USD
  alerts:
    - threshold: 50%   # Alert at $25
    - threshold: 80%   # Alert at $40
    - threshold: 100%  # Alert at $50
    - threshold: 150%  # Critical at $75
```

### Metrics à surveiller

1. **request_count** (requêtes/jour)
   - Alerte si > 200K/jour sans raison

2. **billable_instance_time** (vCPU-secondes)
   - Alerte si > 1M vCPU-s/jour

3. **container_memory_utilization** (%)
   - Alerte si > 80% (risque OOM)

4. **response_latencies** (p95, p99)
   - Alerte si p95 > 5s (trop de cache MISS)

---

## 📊 ROI Analysis

### Coût vs Alternative (serveur dédié Hetzner)

| Solution | Coût/mois | Maintenance | Scalabilité | Disponibilité |
|----------|-----------|-------------|-------------|---------------|
| **Cloud Run** | $15-80 | ✅ Zéro | ✅ Auto | 99.95% |
| **Hetzner CX11** | €4.51 (~$5) | ❌ Manuelle | ❌ Manuelle | 99.9% |
| **Hetzner CX21** | €5.99 (~$6.50) | ❌ Manuelle | ❌ Manuelle | 99.9% |

**Pour < 2M req/mois**: Cloud Run plus rentable (free tier + zéro maintenance)

**Pour 2-10M req/mois**: Cloud Run compétitif si vous valorisez :
- ✅ Zéro DevOps/maintenance
- ✅ Auto-scaling
- ✅ Pay-per-use
- ✅ SLA 99.95%

**Pour > 20M req/mois**: Considérer :
- Compute Engine + Load Balancer
- Kubernetes (GKE Autopilot)
- Hybrid: Cloud Run + CDN (Cloudflare)

---

## 🎁 Tips pour réduire les coûts

### 1. Activer HTTP/2 et compression
```yaml
# cloud-run.yaml
compression: true
http2: true
```
**Économie réseau**: -40% (2 KB → 1.2 KB par réponse)

### 2. Implémenter ETag/Cache-Control
```go
w.Header().Set("Cache-Control", "public, max-age=60")
w.Header().Set("ETag", fmt.Sprintf(`"%d"`, lastRefresh.Unix()))
```
**Économie**: -20% requêtes (clients utilisent cache local)

### 3. Rate Limiting côté client
```go
// Clients SDK: cache local + rate limit
```
**Économie**: -30% requêtes inutiles

### 4. Monitoring intelligent
- Scale to zero la nuit si pas de traffic
- Scheduled tasks pour pre-warm aux heures de pointe
- Cloud Scheduler pour health checks intelligents

---

## 📞 Support & Monitoring

### Outils recommandés (tous gratuits ou inclus)

1. **Google Cloud Monitoring** - Métriques built-in
2. **Google Cloud Logging** - Logs structurés
3. **Google Cloud Trace** - Latency analysis
4. **Uptime Checks** (100 checks/mois gratuit)
5. **Budget Alerts** - Notifications par email/Slack

### Dashboard metrics clés

```
1. Request count (par endpoint)
2. Request latency (p50, p95, p99)
3. Error rate (5xx, 4xx)
4. Cache HIT rate (calculé via logs)
5. Memory utilization
6. CPU utilization
7. Cold start count
8. Billable time
```

---

## 🔒 Sécurité incluse (pas de coût additionnel)

- ✅ HTTPS automatique
- ✅ DDoS protection (Cloud Armor Lite)
- ✅ IAM authentication
- ✅ VPC Service Controls (si besoin)
- ✅ Secret Manager (1000 secrets gratuits)
- ✅ Vulnerability scanning (container)

---

## Conclusion

### 💰 Estimation finale pour 1 instance Cloud Run

**Pour un usage réaliste (1-5M req/mois, 80-90% cache HIT):**

```
Coût mensuel estimé: $15-50 USD
Coût par requête: $0.000008-0.000015 USD
```

**Avec votre architecture lazy-loading:**
- ✅ **Scale-to-zero**: $0 quand pas utilisé
- ✅ **Free tier**: Premiers 30-50K req gratuits chaque mois
- ✅ **Pay-per-use**: Vous payez seulement ce que vous consommez
- ✅ **Cache optimisé**: 80-95% cache HIT réduit drastiquement les coûts

**Pour commencer**: Budget $20-30/mois largement suffisant pour les 6 premiers mois.

---

*Dernière mise à jour: Novembre 2024*
*Tarifs basés sur: https://cloud.google.com/run/pricing*
