# Gist Business Activities — Taxonomy Edition 1 (`gist-activities/1`)

- **Status:** Draft reference taxonomy for RFC-002 §11 ("Taxonomy and
  visibility"). Not yet wired into any endpoint; it becomes consumable at M1
  as `GET /v1/taxonomies/gist-activities/editions/1` (shape per §9).
- **Edition:** 1 (`gist-activities/1`). Versioned like a capability contract:
  additive node additions are minor; renaming or removing a node, or changing
  its meaning, is a new edition. Reclassification never changes execution
  permissions (RFC-002 §11).
- **Scope:** Levels 1–3 of APQC PCF v8.0 (Cross-Industry), February 2026:
  13 Level-1 categories, 74 Level-2 process groups, 362 Level-3 processes.
  Levels 4–5 (activities and tasks) are deliberately excluded from this
  edition — they inform capability-contract naming (§ "Seeding core
  capability families" below), not discovery filtering.

## Attribution (required, do not remove)

This edition is a derivative work of the APQC Process Classification
Framework (PCF), distributed under APQC's license permitting use, copying,
publication, modification, and derivative works **provided this attribution
accompanies every copy and derivative**:

> APQC Process Classification Framework (PCF) is an open standard developed
> by APQC, a nonprofit that promotes benchmarking and best practices
> worldwide. The PCF is intended to facilitate organizational improvement
> through process management and benchmarking, regardless of industry, size,
> or geography. To download the full PCF or industry-specific versions of
> the PCF, as well as associated measures and benchmarking, please visit
> www.apqc.org/pcf.

©2026 APQC. All rights reserved in the original work; this derivative is
subject to the license above. The attribution text above must be served with
the taxonomy document at every endpoint that returns taxonomy nodes
(machine-attached, not prose-only), and preserved in any downstream copy.

## Node identity

- Node IDs for this edition are the dotted path (`1.0`, `1.1`, `1.1.1`, …).
  They are identifiers, not ordering guarantees.
- Each node carries the source `apqc_ref` number (APQC's stable 5-digit
  reference) for traceability back to PCF v8.0. Node IDs are Gist-owned and
  stable across editions; `apqc_ref` is provenance metadata, not identity.
- Titles are carried over with light normalization only. Where this edition
  deviates from PCF wording in a later edition, the change is recorded
  per-node.

## What this taxonomy is for

Discovery filtering, catalog organization, and publisher tagging
(RFC-002 §8, §11). It is **not**:

- A capability-contract namespace. Capability IDs (`gist/…`, `{workspace}/…`)
  are governed contracts (§6, §11) and are independent of taxonomy paths;
  taxonomy membership is optional metadata.
- A permission boundary. Membership grants nothing.

### 1.0 Develop Vision and Strategy
`apqc_ref` 10002 · Level 1 category

#### 1.1 Define the business concept and long-term vision

`apqc_ref` 17040 · Level 2 process group
- **1.1.1 Assess the external environment** (`apqc_ref` 10017)
- **1.1.2 Survey market and determine customer needs and wants** (`apqc_ref` 10018)
- **1.1.3 Assess the internal environment** (`apqc_ref` 10019)
- **1.1.4 Establish strategic vision** (`apqc_ref` 10020)
- **1.1.5 Conduct organization restructuring opportunities** (`apqc_ref` 16792)

#### 1.2 Develop business strategy

`apqc_ref` 10015 · Level 2 process group
- **1.2.1 Develop overall mission statement** (`apqc_ref` 10037)
- **1.2.2 Define and evaluate strategic options to achieve the mission** (`apqc_ref` 10038)
- **1.2.3 Set/Develop long-term enterprise strategy** (`apqc_ref` 10039)
- **1.2.4 Coordinate and align cross-functional and process strategies** (`apqc_ref` 10040)
- **1.2.5 Create organizational design** (`apqc_ref` 10041)
- **1.2.6 Develop and set organizational objectives** (`apqc_ref` 10042)
- **1.2.7 Formulate business unit strategies** (`apqc_ref` 10043)
- **1.2.8 Develop customer experience strategy** (`apqc_ref` 19959)
- **1.2.9 Communicate strategies internally and externally** (`apqc_ref` 18916)

#### 1.3 Develop and measure strategic initiatives

`apqc_ref` 10016 · Level 2 process group
- **1.3.1 Develop strategic initiatives** (`apqc_ref` 10057)
- **1.3.2 Evaluate strategic initiatives** (`apqc_ref` 10058)
- **1.3.3 Select strategic initiatives** (`apqc_ref` 10059)
- **1.3.4 Establish high-level measures** (`apqc_ref` 10060)
- **1.3.5 Execute strategic initiatives** (`apqc_ref` 19507)
- **1.3.6 Review execution of strategic initiatives** (`apqc_ref` 21422)
- **1.3.7 Refine strategic initiatives and project plans as needed** (`apqc_ref` 21423)

#### 1.4 Develop and maintain business models

`apqc_ref` 20944 · Level 2 process group
- **1.4.1 Develop business models** (`apqc_ref` 20945)
- **1.4.2 Maintain business models** (`apqc_ref` 20950)
- **1.4.3 Establish business model governance** (`apqc_ref` 20955)

### 2.0 Develop and Manage Products and Services
`apqc_ref` 10003 · Level 1 category

#### 2.1 Govern and manage product/service development program

`apqc_ref` 19696 · Level 2 process group
- **2.1.1 Manage product and service portfolio** (`apqc_ref` 10061)
- **2.1.2 Manage product and service life cycle** (`apqc_ref` 10067)
- **2.1.3 Manage patents, copyrights, and regulatory requirements** (`apqc_ref` 19985)
- **2.1.4 Manage product and service master data** (`apqc_ref` 11740)

#### 2.2 Generate and define new product/service ideas

`apqc_ref` 19698 · Level 2 process group
- **2.2.1 Perform discovery research** (`apqc_ref` 10065)
- **2.2.2 Generate new product/service concepts** (`apqc_ref` 19669)
- **2.2.3 Define product/service development requirements** (`apqc_ref` 19990)

#### 2.3 Develop products and services

`apqc_ref` 10062 · Level 2 process group
- **2.3.1 Design and prototype products and services** (`apqc_ref` 19993)
- **2.3.2 Test market for new or revised products and services** (`apqc_ref` 19996)
- **2.3.3 Prepare for production/service delivery** (`apqc_ref` 19997)
- **2.3.4 Define warranty claims** (`apqc_ref` 20089)
- **2.3.5 Develop recall strategy** (`apqc_ref` 20092)

### 3.0 Market and Sell Products and Services
`apqc_ref` 10004 · Level 1 category

#### 3.1 Understand markets, customers, and capabilities

`apqc_ref` 10101 · Level 2 process group
- **3.1.1 Perform customer and market intelligence analysis** (`apqc_ref` 10106)
- **3.1.2 Evaluate and prioritize market opportunities** (`apqc_ref` 10107)

#### 3.2 Develop marketing strategy

`apqc_ref` 21615 · Level 2 process group
- **3.2.1 Define offering and customer value proposition** (`apqc_ref` 11168)
- **3.2.2 Develop and manage brands** (`apqc_ref` 11445)
- **3.2.3 Define pricing strategy** (`apqc_ref` 10123)
- **3.2.4 Define and manage channel strategy** (`apqc_ref` 20000)
- **3.2.5 Analyze and manage channel performance** (`apqc_ref` 20006)
- **3.2.6 Develop marketing communication strategy** (`apqc_ref` 16848)
- **3.2.7 Design and manage customer loyalty program** (`apqc_ref` 18924)

#### 3.3 Develop and manage marketing plans

`apqc_ref` 20008 · Level 2 process group
- **3.3.1 Establish goals, objectives, and measures for products/services by channel/segment** (`apqc_ref` 10148)
- **3.3.2 Establish marketing budgets** (`apqc_ref` 10149)
- **3.3.3 Develop and manage pricing** (`apqc_ref` 20593)
- **3.3.4 Develop and manage promotional activities** (`apqc_ref` 20010)
- **3.3.5 Track customer management measures** (`apqc_ref` 10153)
- **3.3.6 Analyze and respond to customer insight** (`apqc_ref` 16613)
- **3.3.7 Develop and manage packaging strategy** (`apqc_ref` 10154)
- **3.3.8 Develop go-to-market strategy** (`apqc_ref` 21425)
- **3.3.9 Manage product marketing material** (`apqc_ref` 16629)

#### 3.4 Develop sales strategy

`apqc_ref` 10103 · Level 2 process group
- **3.4.1 Develop sales forecast** (`apqc_ref` 10129)
- **3.4.2 Develop sales partner/alliance relationships** (`apqc_ref` 10130)
- **3.4.3 Establish overall sales budgets** (`apqc_ref` 10131)
- **3.4.4 Establish sales goals and measures** (`apqc_ref` 10132)
- **3.4.5 Establish customer management measures** (`apqc_ref` 10133)

#### 3.5 Develop and manage sales plans

`apqc_ref` 10105 · Level 2 process group
- **3.5.1 Manage leads/opportunities** (`apqc_ref` 10182)
- **3.5.2 Manage customers and accounts** (`apqc_ref` 10183)
- **3.5.3 Develop and manage sales proposals, bids, and quotes** (`apqc_ref` 11779)
- **3.5.4 Manage sales orders** (`apqc_ref` 10185)
- **3.5.5 Manage sales partners and alliances** (`apqc_ref` 10187)
- **3.5.6 Perform sales at physical outlets** (`apqc_ref` 21427)
- **3.5.7 Perform field sales** (`apqc_ref` 21428)
- **3.5.8 Perform digital sales** (`apqc_ref` 21429)

### 4.0 Manage Supply Chain for Physical Products
`apqc_ref` 20022 · Level 1 category

#### 4.1 Plan for and align supply chain resources

`apqc_ref` 10215 · Level 2 process group
- **4.1.1 Develop production and materials strategies** (`apqc_ref` 10221)
- **4.1.2 Manage demand for products** (`apqc_ref` 10222)
- **4.1.3 Create materials plan** (`apqc_ref` 10223)
- **4.1.4 Create and manage master production schedule** (`apqc_ref` 10224)
- **4.1.5 Plan distribution requirements** (`apqc_ref` 17042)
- **4.1.6 Establish distribution planning constraints** (`apqc_ref` 10226)
- **4.1.7 Review distribution planning policies** (`apqc_ref` 10227)
- **4.1.8 Develop quality standards and procedures** (`apqc_ref` 10368)

#### 4.2 Procure materials and services

`apqc_ref` 10216 · Level 2 process group
- **4.2.1 Provide sourcing governance** (`apqc_ref` 10277)
- **4.2.2 Develop category plan** (`apqc_ref` 16636)
- **4.2.3 Select suppliers and develop/maintain contracts** (`apqc_ref` 10278)
- **4.2.4 Order materials and services** (`apqc_ref` 10279)
- **4.2.5 Manage suppliers** (`apqc_ref` 10280)

#### 4.3 Produce/Assemble/Test product

`apqc_ref` 10217 · Level 2 process group
- **4.3.1 Schedule production** (`apqc_ref` 10303)
- **4.3.2 Produce/Assemble product** (`apqc_ref` 10304)
- **4.3.3 Perform quality testing** (`apqc_ref` 10369)
- **4.3.4 Maintain production records and manage lot traceability** (`apqc_ref` 10370)

#### 4.4 Manage logistics and warehousing

`apqc_ref` 10219 · Level 2 process group
- **4.4.1 Provide logistics governance** (`apqc_ref` 10338)
- **4.4.2 Plan and manage inbound material flow** (`apqc_ref` 20936)
- **4.4.3 Operate warehousing** (`apqc_ref` 10340)
- **4.4.4 Operate outbound transportation** (`apqc_ref` 10341)
- **4.4.5 Deliver last mile delivery services** (`apqc_ref` 21617)

### 5.0 Deliver Services
`apqc_ref` 20025 · Level 1 category

#### 5.1 Establish service delivery governance and strategies

`apqc_ref` 20026 · Level 2 process group
- **5.1.1 Establish service delivery governance** (`apqc_ref` 20027)
- **5.1.2 Develop service delivery strategies** (`apqc_ref` 20032)

#### 5.2 Manage service delivery resources

`apqc_ref` 20040 · Level 2 process group
- **5.2.1 Manage service delivery resource demand** (`apqc_ref` 20041)
- **5.2.2 Create and manage resource plan** (`apqc_ref` 20050)
- **5.2.3 Enable service delivery resources** (`apqc_ref` 12127)

#### 5.3 Manage and Operate Service Delivery System

`apqc_ref` 21634 · Level 2 process group
- **5.3.1 Perform service delivery system planning** (`apqc_ref` 21635)
- **5.3.2 Perform service delivery system startup** (`apqc_ref` 21636)
- **5.3.3 Operate service delivery system** (`apqc_ref` 21637)
- **5.3.4 Assure service delivery system service** (`apqc_ref` 21638)

#### 5.4 Deliver service to customer

`apqc_ref` 20058 · Level 2 process group
- **5.4.1 Initiate service delivery** (`apqc_ref` 20059)
- **5.4.2 Execute service delivery** (`apqc_ref` 20069)
- **5.4.3 Complete service delivery** (`apqc_ref` 20077)

### 6.0 Manage Customer Service
`apqc_ref` 20085 · Level 1 category

#### 6.1 Develop customer service strategy

`apqc_ref` 10378 · Level 2 process group
- **6.1.1 Define customer service requirements across the enterprise** (`apqc_ref` 20086)
- **6.1.2 Define customer service experience** (`apqc_ref` 20087)
- **6.1.3 Define and manage customer service channel strategy** (`apqc_ref` 20088)
- **6.1.4 Define customer service policies and procedures** (`apqc_ref` 10382)
- **6.1.5 Establish target service level for each customer segment** (`apqc_ref` 10383)
- **6.1.6 Identify and define customer service strategy integrations** (`apqc_ref` 21707)

#### 6.2 Plan and manage customer service contacts

`apqc_ref` 10379 · Level 2 process group
- **6.2.1 Plan and manage customer service workforce** (`apqc_ref` 10387)
- **6.2.2 Manage customer service problems, requests, and inquiries** (`apqc_ref` 10388)
- **6.2.3 Manage customer complaints** (`apqc_ref` 10389)
- **6.2.4 Process returns** (`apqc_ref` 20094)
- **6.2.5 Report incidents and risks to regulatory bodies** (`apqc_ref` 12840)

#### 6.3 Service products after sales

`apqc_ref` 12658 · Level 2 process group
- **6.3.1 Register products** (`apqc_ref` 20605)
- **6.3.2 Process warranty claims** (`apqc_ref` 12669)
- **6.3.3 Manage supplier recovery** (`apqc_ref` 20106)
- **6.3.4 Service products** (`apqc_ref` 10218)

#### 6.4 Manage product recalls and regulatory audits

`apqc_ref` 20110 · Level 2 process group
- **6.4.1 Initiate recall** (`apqc_ref` 20111)
- **6.4.2 Assess the likelihood and consequences of occurrence of any hazards** (`apqc_ref` 20112)
- **6.4.3 Manage recall related communications** (`apqc_ref` 20113)
- **6.4.4 Submit regulatory reports** (`apqc_ref` 20114)
- **6.4.5 Monitor and audit recall effectiveness** (`apqc_ref` 20115)
- **6.4.6 Manage recall termination** (`apqc_ref` 20116)

#### 6.5 Evaluate customer service operations and customer satisfacion

`apqc_ref` 20595 · Level 2 process group
- **6.5.1 Measure customer satisfaction with customer problems, requests, and inquiries handling** (`apqc_ref` 10401)
- **6.5.2 Measure customer satisfaction with customer-complaint handling and resolution** (`apqc_ref` 10402)
- **6.5.3 Measure customer satisfaction with products and services** (`apqc_ref` 10403)
- **6.5.4 Evaluate and manage warranty performance** (`apqc_ref` 12672)
- **6.5.5 Evaluate recall performance** (`apqc_ref` 20121)

### 7.0 Develop and Manage Human Resources
`apqc_ref` 10007 · Level 1 category

#### 7.1 Develop and manage human resources (HR) planning, policies, and strategies

`apqc_ref` 17043 · Level 2 process group
- **7.1.1 Develop human resources strategy** (`apqc_ref` 20958)
- **7.1.2 Develop and implement workforce planning, policies, and strategies** (`apqc_ref` 17045)
- **7.1.3 Monitor and update HR strategies, plans, and policies** (`apqc_ref` 10417)
- **7.1.4 Develop competency management models** (`apqc_ref` 17046)

#### 7.2 Recruit, source, and select employees

`apqc_ref` 10410 · Level 2 process group
- **7.2.1 Manage employee requisitions** (`apqc_ref` 21698)
- **7.2.2 Recruit/Source candidates** (`apqc_ref` 10440)
- **7.2.3 Screen and select candidates** (`apqc_ref` 20123)
- **7.2.4 Manage new hire/re-hire** (`apqc_ref` 10443)
- **7.2.5 Manage applicant information** (`apqc_ref` 10444)

#### 7.3 Manage employee onboarding, training, and development

`apqc_ref` 20599 · Level 2 process group
- **7.3.1 Manage employee onboarding** (`apqc_ref` 10469)
- **7.3.2 Manage employee performance** (`apqc_ref` 10470)
- **7.3.3 Manage employee career development** (`apqc_ref` 10472)
- **7.3.4 Develop and train employees** (`apqc_ref` 10473)

#### 7.4 Manage employee relations

`apqc_ref` 17052 · Level 2 process group
- **7.4.1 Manage labor relations** (`apqc_ref` 10483)
- **7.4.2 Manage collective bargaining process** (`apqc_ref` 10484)
- **7.4.3 Manage labor management partnerships** (`apqc_ref` 10485)
- **7.4.4 Manage employee grievances** (`apqc_ref` 10531)
- **7.4.5 Monitor legal and regulatory environment** (`apqc_ref` 21437)

#### 7.5 Reward and retain employees

`apqc_ref` 10412 · Level 2 process group
- **7.5.1 Develop and manage reward, recognition, and motivation programs** (`apqc_ref` 21438)
- **7.5.2 Manage and administer benefits** (`apqc_ref` 10495)
- **7.5.3 Manage employee assistance and retention** (`apqc_ref` 21439)
- **7.5.4 Administer payroll** (`apqc_ref` 10497)

#### 7.6 Redeploy and retire employees

`apqc_ref` 10413 · Level 2 process group
- **7.6.1 Manage promotion and demotion process** (`apqc_ref` 10512)
- **7.6.2 Manage separation** (`apqc_ref` 10513)
- **7.6.3 Relocate employees and manage assignments** (`apqc_ref` 17055)

#### 7.7 Manage employee information and analytics

`apqc_ref` 17056 · Level 2 process group
- **7.7.1 Manage reporting processes** (`apqc_ref` 10522)
- **7.7.2 Manage employee inquiry process** (`apqc_ref` 10523)
- **7.7.3 Manage and maintain employee data** (`apqc_ref` 10524)
- **7.7.4 Manage human resource information systems** (`apqc_ref` 10525)
- **7.7.5 Develop and manage employee measures** (`apqc_ref` 10526)
- **7.7.6 Develop and manage time and attendance systems** (`apqc_ref` 10527)
- **7.7.7 Develop workforce analytics** (`apqc_ref` 21441)
- **7.7.8 Implement workforce analytics** (`apqc_ref` 21447)
- **7.7.9 Manage/Collect employee suggestions and perform employee research** (`apqc_ref` 10530)

#### 7.8 Manage employee communication

`apqc_ref` 21451 · Level 2 process group
- **7.8.1 Develop employee communication plan** (`apqc_ref` 10529)
- **7.8.2 Conduct employee engagement surveys** (`apqc_ref` 16944)
- **7.8.3 Deliver employee communications** (`apqc_ref` 10532)

### 8.0 Manage Information Technology (IT)
`apqc_ref` 20607 · Level 1 category

#### 8.1 Develop and manage IT customer relationships

`apqc_ref` 20608 · Level 2 process group
- **8.1.1 Understand IT customer needs** (`apqc_ref` 20609)
- **8.1.2 Identify IT customer transformation needs** (`apqc_ref` 20612)
- **8.1.3 Plan and communicate IT services** (`apqc_ref` 20617)
- **8.1.4 Provide IT transformation guidance** (`apqc_ref` 20623)
- **8.1.5 Develop and manage IT service levels** (`apqc_ref` 20632)
- **8.1.6 Manage IT customer relationships** (`apqc_ref` 20641)
- **8.1.7 Analyze service performance** (`apqc_ref` 20648)

#### 8.2 Develop and manage IT business strategy

`apqc_ref` 20652 · Level 2 process group
- **8.2.1 Define business technology and governance strategy** (`apqc_ref` 20653)
- **8.2.2 Manage IT portfolio strategy** (`apqc_ref` 20660)
- **8.2.3 Define and maintain enterprise architecture** (`apqc_ref` 20668)
- **8.2.4 Define IT service management strategy** (`apqc_ref` 20674)
- **8.2.5 Control IT management system** (`apqc_ref` 20682)
- **8.2.6 Manage IT value portfolio** (`apqc_ref` 20693)
- **8.2.7 Define and manage technology innovation** (`apqc_ref` 20699)

#### 8.3 Develop and manage IT resilience and risk

`apqc_ref` 20706 · Level 2 process group
- **8.3.1 Develop IT compliance, risk, and security strategy** (`apqc_ref` 20707)
- **8.3.2 Develop IT resilience strategy** (`apqc_ref` 20716)
- **8.3.3 Control IT risk, compliance, and security** (`apqc_ref` 20721)
- **8.3.4 Plan and manage IT continuity** (`apqc_ref` 20731)
- **8.3.5 Develop and manage IT security, privacy, and data protection** (`apqc_ref` 20735)
- **8.3.6 Conduct and analyze IT compliance assessments** (`apqc_ref` 20743)
- **8.3.7 Develop and execute IT resilience and continuity operations** (`apqc_ref` 20749)
- **8.3.8 Manage IT user identity and authorization** (`apqc_ref` 20756)

#### 8.4 Manage information

`apqc_ref` 20765 · Level 2 process group
- **8.4.1 Define business information and analytics strategy** (`apqc_ref` 20766)
- **8.4.2 Define and maintain business information architecture** (`apqc_ref` 20770)
- **8.4.3 Define and execute business information lifecycle planning and control** (`apqc_ref` 20776)
- **8.4.4 Manage business information** (`apqc_ref` 20779)

#### 8.5 Develop and manage services/solutions

`apqc_ref` 20784 · Level 2 process group
- **8.5.1 Develop service/solution and integration strategy** (`apqc_ref` 20785)
- **8.5.2 Manage service/solution lifecycle planning** (`apqc_ref` 20793)
- **8.5.3 Develop and manage service/solution architecture** (`apqc_ref` 20799)
- **8.5.4 Execute IT service/solution creation and testing** (`apqc_ref` 20808)
- **8.5.5 Perform service/solution maintenance and testing** (`apqc_ref` 20817)

#### 8.6 Deploy services/solutions

`apqc_ref` 20824 · Level 2 process group
- **8.6.1 Develop and manage service/solution deployment strategy** (`apqc_ref` 20825)
- **8.6.2 Plan service and solution implementation** (`apqc_ref` 20832)
- **8.6.3 Manage change deployment control** (`apqc_ref` 20840)
- **8.6.4 Implement technology solutions** (`apqc_ref` 20848)
- **8.6.5 Perform service and solution rollout** (`apqc_ref` 20858)

#### 8.7 Create and manage support services/solutions

`apqc_ref` 20866 · Level 2 process group
- **8.7.1 Define and establish service delivery strategy** (`apqc_ref` 20867)
- **8.7.2 Define and develop service support strategy** (`apqc_ref` 20873)
- **8.7.3 Plan and manage service delivery control** (`apqc_ref` 20880)
- **8.7.4 Develop and manage infrastructure resource planning** (`apqc_ref` 20888)
- **8.7.5 Define service support planning** (`apqc_ref` 20895)
- **8.7.6 Develop and manage service delivery operations** (`apqc_ref` 20905)
- **8.7.7 Manage infrastructure resource administration** (`apqc_ref` 20914)
- **8.7.8 Operate IT user support** (`apqc_ref` 20921)

### 9.0 Manage Financial Resources
`apqc_ref` 17058 · Level 1 category

#### 9.1 Perform planning and management accounting

`apqc_ref` 10728 · Level 2 process group
- **9.1.1 Develop and manage financial resources strategy** (`apqc_ref` 21694)
- **9.1.2 Perform planning/budgeting/forecasting** (`apqc_ref` 10738)
- **9.1.3 Perform cost accounting and control** (`apqc_ref` 10739)
- **9.1.4 Perform cost management** (`apqc_ref` 10740)
- **9.1.5 Evaluate and manage financial performance** (`apqc_ref` 10741)

#### 9.2 Perform revenue accounting

`apqc_ref` 10729 · Level 2 process group
- **9.2.1 Process customer credit** (`apqc_ref` 10742)
- **9.2.2 Invoice customer** (`apqc_ref` 10743)
- **9.2.3 Process accounts receivable (AR)** (`apqc_ref` 10744)
- **9.2.4 Manage and process collections** (`apqc_ref` 10745)
- **9.2.5 Manage and process adjustments/deductions** (`apqc_ref` 10746)

#### 9.3 Perform general accounting and reporting

`apqc_ref` 10730 · Level 2 process group
- **9.3.1 Manage financial policies and procedures** (`apqc_ref` 10747)
- **9.3.2 Perform general accounting** (`apqc_ref` 10748)
- **9.3.3 Perform fixed-asset accounting** (`apqc_ref` 10749)
- **9.3.4 Perform financial reporting** (`apqc_ref` 10750)

#### 9.4 Manage fixed-asset project accounting

`apqc_ref` 10731 · Level 2 process group
- **9.4.1 Perform capital planning and project approval** (`apqc_ref` 10751)
- **9.4.2 Perform capital project accounting** (`apqc_ref` 10752)

#### 9.5 Process payroll

`apqc_ref` 10732 · Level 2 process group
- **9.5.1 Report time** (`apqc_ref` 10753)
- **9.5.2 Manage pay** (`apqc_ref` 10754)
- **9.5.3 Manage and process payroll taxes** (`apqc_ref` 10755)

#### 9.6 Process accounts payable and expense reimbursements

`apqc_ref` 10733 · Level 2 process group
- **9.6.1 Process accounts payable (AP)** (`apqc_ref` 10756)
- **9.6.2 Process expense reimbursements** (`apqc_ref` 10757)
- **9.6.3 Manage corporate credit cards** (`apqc_ref` 20929)

#### 9.7 Manage treasury operations

`apqc_ref` 10734 · Level 2 process group
- **9.7.1 Manage treasury policies and procedures** (`apqc_ref` 10758)
- **9.7.2 Manage cash** (`apqc_ref` 10759)
- **9.7.3 Manage in-house bank accounts** (`apqc_ref` 10760)
- **9.7.4 Manage debt and investment** (`apqc_ref` 10761)
- **9.7.5 Monitor and execute risk and hedging transactions** (`apqc_ref` 11208)
- **9.7.6 Manage financial fraud/dispute cases** (`apqc_ref` 16958)

#### 9.8 Manage internal controls

`apqc_ref` 10735 · Level 2 process group
- **9.8.1 Establish internal controls, policies, and procedures** (`apqc_ref` 10762)
- **9.8.2 Operate controls and monitor compliance with internal controls policies and procedures** (`apqc_ref` 21574)
- **9.8.3 Report on internal controls compliance** (`apqc_ref` 10764)

#### 9.9 Manage taxes

`apqc_ref` 10736 · Level 2 process group
- **9.9.1 Develop tax strategy and plan** (`apqc_ref` 10765)
- **9.9.2 Process taxes** (`apqc_ref` 10766)

#### 9.10 Manage international funds/consolidation

`apqc_ref` 10737 · Level 2 process group
- **9.10.1 Monitor international rates** (`apqc_ref` 10767)
- **9.10.2 Manage transactions** (`apqc_ref` 10768)
- **9.10.3 Monitor currency exposure/hedge currency** (`apqc_ref` 10769)
- **9.10.4 Report results** (`apqc_ref` 10770)

#### 9.11 Perform global trade services

`apqc_ref` 17059 · Level 2 process group
- **9.11.1 Screen sanctioned party list** (`apqc_ref` 14090)
- **9.11.2 Control exports and imports** (`apqc_ref` 14091)
- **9.11.3 Classify products** (`apqc_ref` 14092)
- **9.11.4 Perform currency conversion** (`apqc_ref` 19593)
- **9.11.5 Calculate duty** (`apqc_ref` 14093)
- **9.11.6 Communicate with customs** (`apqc_ref` 14094)
- **9.11.7 Document trade** (`apqc_ref` 14095)
- **9.11.8 Process trade preferences** (`apqc_ref` 14096)
- **9.11.9 Handle restitution** (`apqc_ref` 14097)
- **9.11.10 Prepare letter of credit** (`apqc_ref` 14098)

### 10.0 Acquire, Construct, and Manage Assets
`apqc_ref` 19207 · Level 1 category

#### 10.1 Plan and acquire assets

`apqc_ref` 10937 · Level 2 process group
- **10.1.1 Develop property strategy and long term vision** (`apqc_ref` 10941)
- **10.1.2 Plan facility** (`apqc_ref` 10943)
- **10.1.3 Provide workspace and facilities** (`apqc_ref` 10944)
- **10.1.4 Manage facilities operations** (`apqc_ref` 10949)

#### 10.2 Design and construct assets

`apqc_ref` 21575 · Level 2 process group
- **10.2.1 Manage capital program for assets** (`apqc_ref` 19209)
- **10.2.2 Design and plan asset construction** (`apqc_ref` 20139)
- **10.2.3 Schedule and perform construction work** (`apqc_ref` 19229)
- **10.2.4 Manage asset construction** (`apqc_ref` 19224)

#### 10.3 Maintain assets

`apqc_ref` 19238 · Level 2 process group
- **10.3.1 Plan asset maintenance** (`apqc_ref` 19239)
- **10.3.2 Manage asset maintenance** (`apqc_ref` 19245)
- **10.3.3 Perform asset maintenance** (`apqc_ref` 19253)

#### 10.4 Manage asset end-of-life

`apqc_ref` 21576 · Level 2 process group
- **10.4.1 Develop exit strategy** (`apqc_ref` 10952)
- **10.4.2 Decomission productive assets** (`apqc_ref` 19258)
- **10.4.3 Perform sale or trade** (`apqc_ref` 10953)
- **10.4.4 Manage take-back centers** (`apqc_ref` 12717)
- **10.4.5 Dismantle assets** (`apqc_ref` 21579)
- **10.4.6 Track parts** (`apqc_ref` 21580)
- **10.4.7 Recycle parts** (`apqc_ref` 21581)
- **10.4.8 Ship hazardous material** (`apqc_ref` 12721)
- **10.4.9 Provide government reporting** (`apqc_ref` 12722)
- **10.4.10 Perform abandonment** (`apqc_ref` 21582)
- **10.4.11 Perform waste and hazardous goods management** (`apqc_ref` 21583)

### 11.0 Manage Enterprise Risk, Compliance, Remediation, and Resiliency
`apqc_ref` 16437 · Level 1 category

#### 11.1 Manage enterprise risk

`apqc_ref` 17060 · Level 2 process group
- **11.1.1 Establish the enterprise risk framework and policies** (`apqc_ref` 16439)
- **11.1.2 Oversee and coordinate enterprise risk management activities** (`apqc_ref` 16445)
- **11.1.3 Manage business unit and function risk** (`apqc_ref` 17462)

#### 11.2 Manage compliance

`apqc_ref` 17467 · Level 2 process group
- **11.2.1 Establish compliance framework and policies** (`apqc_ref` 17468)
- **11.2.2 Manage regulatory compliance** (`apqc_ref` 16463)

#### 11.3 Manage remediation efforts

`apqc_ref` 11185 · Level 2 process group
- **11.3.1 Create remediation plans** (`apqc_ref` 11201)
- **11.3.2 Contact and confer with experts** (`apqc_ref` 11202)
- **11.3.3 Identify/dedicate resources** (`apqc_ref` 11203)
- **11.3.4 Investigate legal aspects** (`apqc_ref` 11204)
- **11.3.5 Investigate damage cause** (`apqc_ref` 11205)
- **11.3.6 Amend or create policy** (`apqc_ref` 11206)

#### 11.4 Manage business resiliency

`apqc_ref` 11216 · Level 2 process group
- **11.4.1 Develop the business resilience strategy** (`apqc_ref` 11221)
- **11.4.2 Perform continuous business operations planning** (`apqc_ref` 11222)
- **11.4.3 Test continuous business operations** (`apqc_ref` 11223)
- **11.4.4 Maintain continuous business operations** (`apqc_ref` 11224)
- **11.4.5 Share knowledge of specific risks across other parts of the organization** (`apqc_ref` 16471)

### 12.0 Manage External Relationships
`apqc_ref` 10012 · Level 1 category

#### 12.1 Build investor relationships

`apqc_ref` 11010 · Level 2 process group
- **12.1.1 Plan, build, and manage lender relations** (`apqc_ref` 11035)
- **12.1.2 Plan, build, and manage analyst relations** (`apqc_ref` 11036)
- **12.1.3 Communicate with shareholders** (`apqc_ref` 11037)

#### 12.2 Manage government and industry relationships

`apqc_ref` 11011 · Level 2 process group
- **12.2.1 Manage government relations** (`apqc_ref` 11038)
- **12.2.2 Manage relations with quasi-government bodies** (`apqc_ref` 11039)
- **12.2.3 Manage relations with trade or industry groups** (`apqc_ref` 11040)
- **12.2.4 Manage lobby activities** (`apqc_ref` 11041)

#### 12.3 Manage relations with board of directors

`apqc_ref` 11012 · Level 2 process group
- **12.3.1 Report financial results** (`apqc_ref` 11042)
- **12.3.2 Report audit findings** (`apqc_ref` 11043)

#### 12.4 Manage legal and ethical issues

`apqc_ref` 11013 · Level 2 process group
- **12.4.1 Create ethics policies** (`apqc_ref` 11044)
- **12.4.2 Manage corporate governance policies** (`apqc_ref` 11045)
- **12.4.3 Develop and perform preventive law programs** (`apqc_ref` 11046)
- **12.4.4 Ensure compliance** (`apqc_ref` 11047)
- **12.4.5 Manage outside counsel** (`apqc_ref` 11048)
- **12.4.6 Protect intellectual property** (`apqc_ref` 11049)
- **12.4.7 Resolve disputes and litigations** (`apqc_ref` 11050)
- **12.4.8 Provide legal advice/counseling** (`apqc_ref` 11051)
- **12.4.9 Negotiate and document agreements/contracts** (`apqc_ref` 11052)

#### 12.5 Manage public relations program

`apqc_ref` 11014 · Level 2 process group
- **12.5.1 Manage community relations** (`apqc_ref` 11066)
- **12.5.2 Manage media relations** (`apqc_ref` 11067)
- **12.5.3 Promote political stability** (`apqc_ref` 11068)
- **12.5.4 Create press releases** (`apqc_ref` 11069)
- **12.5.5 Issue press releases** (`apqc_ref` 11070)

### 13.0 Develop and Manage Business Capabilities
`apqc_ref` 10013 · Level 1 category

#### 13.1 Manage business processes

`apqc_ref` 16378 · Level 2 process group
- **13.1.1 Establish and maintain process management governance** (`apqc_ref` 16379)
- **13.1.2 Define and manage process frameworks** (`apqc_ref` 16384)
- **13.1.3 Define processes** (`apqc_ref` 16387)
- **13.1.4 Manage process performance** (`apqc_ref` 16392)
- **13.1.5 Improve processes** (`apqc_ref` 21453)

#### 13.2 Manage portfolio, program, and project

`apqc_ref` 16400 · Level 2 process group
- **13.2.1 Manage portfolio** (`apqc_ref` 16401)
- **13.2.2 Manage programs** (`apqc_ref` 16405)
- **13.2.3 Manage projects** (`apqc_ref` 16410)

#### 13.3 Manage enterprise quality

`apqc_ref` 17471 · Level 2 process group
- **13.3.1 Establish quality requirements** (`apqc_ref` 17472)
- **13.3.2 Evaluate performance to requirements** (`apqc_ref` 17482)
- **13.3.3 Manage non-conformance** (`apqc_ref` 17492)
- **13.3.4 Implement and maintain the enterprise quality management system (EQMS)** (`apqc_ref` 17498)

#### 13.4 Manage change

`apqc_ref` 11074 · Level 2 process group
- **13.4.1 Plan for change** (`apqc_ref` 21457)
- **13.4.2 Design the change** (`apqc_ref` 11135)
- **13.4.3 Implement change** (`apqc_ref` 11136)
- **13.4.4 Sustain improvement** (`apqc_ref` 11137)

#### 13.5 Develop and manage enterprise-wide knowledge management (KM) capability

`apqc_ref` 11073 · Level 2 process group
- **13.5.1 Develop KM strategy** (`apqc_ref` 11095)
- **13.5.2 Assess KM capabilities** (`apqc_ref` 11096)
- **13.5.3 Design and implement KM capabilities** (`apqc_ref` 20965)
- **13.5.4 Evolve and sustain KM capabilities** (`apqc_ref` 20969)

#### 13.6 Manage Content

`apqc_ref` 21646 · Level 2 process group
- **13.6.1 Define content management strategy** (`apqc_ref` 21647)
- **13.6.2 Develop and manage taxonomies** (`apqc_ref` 21656)
- **13.6.3 Define content systems of record and storage requirements** (`apqc_ref` 21660)
- **13.6.4 Manage content infrastructure** (`apqc_ref` 21663)
- **13.6.5 Develop and manage content** (`apqc_ref` 21670)
- **13.6.6 Deliver approved content** (`apqc_ref` 21679)
- **13.6.7 Control delivered content** (`apqc_ref` 21683)

#### 13.7 Measure and benchmark

`apqc_ref` 21584 · Level 2 process group
- **13.7.1 Define and manage organizational performance strategy** (`apqc_ref` 21585)
- **13.7.2 Benchmark performance** (`apqc_ref` 11072)
- **13.7.3 Evaluate performance** (`apqc_ref` 20147)

#### 13.8 Develop, manage, and deliver analytics

`apqc_ref` 20959 · Level 2 process group
- **13.8.1 Identify needs from stakeholders** (`apqc_ref` 21459)
- **13.8.2 Scope analytics project** (`apqc_ref` 21460)
- **13.8.3 Develop and manage hypotheses** (`apqc_ref` 20960)
- **13.8.4 Collect data** (`apqc_ref` 20961)
- **13.8.5 Prepare data** (`apqc_ref` 21461)
- **13.8.6 Analyze data** (`apqc_ref` 20962)
- **13.8.7 Create data models** (`apqc_ref` 21462)
- **13.8.8 Review data models with stakeholders** (`apqc_ref` 21463)
- **13.8.9 Refine data models** (`apqc_ref` 21464)
- **13.8.10 Report on analysis** (`apqc_ref` 20963)
- **13.8.11 Identify remedial actions** (`apqc_ref` 20964)

#### 13.9 Manage environmental health and safety (EHS)

`apqc_ref` 11179 · Level 2 process group
- **13.9.1 Determine environmental health and safety impacts** (`apqc_ref` 11180)
- **13.9.2 Develop EHS program** (`apqc_ref` 11181)
- **13.9.3 Monitor and manage EHS program** (`apqc_ref` 21587)

#### 13.10 Manage sustainability

`apqc_ref` 21588 · Level 2 process group
- **13.10.1 Develop sustainability capability** (`apqc_ref` 21589)
- **13.10.2 Manage sustainability capability** (`apqc_ref` 21598)
## Seeding core capability families (naming guidance, not identity)

The taxonomy seeds which `gist/…` core capability contracts get defined
first, by activity density in the PCF Level-4 layer (1,357 activities).
Capability IDs remain governed contracts named by the Gist operators
(RFC-002 §6/§11); this section is the naming-guidance input David's ruling
assigned to this taxonomy, not a dependency. Recommended first M1 families,
with the activity clusters that justify them:

| Candidate family | Example core contracts | L4 activity evidence |
| --- | --- | --- |
| `identity` | `identity/user.create`, `identity/account.update`, `identity/access.grant` | 33 account activities; onboarding/offboarding; access/permission management across HR and IT categories |
| `communication` | `communication/channel.invite`, `communication/message.send`, `communication/notification.send` | Notification/send activities across customer service, HR, external-relationship categories |
| `document` | `document/parse`, `document/extract`, `document/generate`, `document/archive` | Parse/extract/generate activities in finance (invoicing), HR (resumes), IT (records) |
| `payment` | `payment/invoice.create`, `payment/payment.process`, `payment/refund.issue` | 16 pay activities, invoicing/billing in the financial-resources category |
| `case` | `case/ticket.create`, `case/ticket.resolve`, `case/escalation.open` | 38 support activities; resolve/respond/complaint clusters in customer service |
| `approval` | `approval/request.submit`, `approval/decision.record` | 17 approve + 9 approval activities spanning HR, finance, IT |
| `schedule` | `schedule/event.create`, `schedule/meeting.plan` | 17 schedule activities across strategy, HR, IT |
| `report` | `report/generate`, `report/analyze` | 41 report activities — the densest verb cluster |
| `record` | `record/track`, `record/audit`, `record/retain` | 19 record + 19 audit activities; retention/backup clusters in IT |
| `deploy` | `deploy/service.provision`, `deploy/change.execute` | 18 deploy activities in the IT category |

Families **not** seeded from L4 density (operator judgment instead):
`gist/identity/user.create` remains the canonical example because identity
creation is the smallest generally-useful mutation; the strategy (§1.0) and
business-capability (§13.0) categories classify skills but rarely mint core
capability contracts — their activities are judgment work, not tool calls.
