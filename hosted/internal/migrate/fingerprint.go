package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
)

// Fingerprints contain definitions and ACL metadata, never table row values.
// The ledger is excluded so adoption does not itself change the schema hash.
const fingerprintSQL = `
SELECT json_build_array('relation',c.relname,c.relkind,c.relpersistence,c.relispartition,pg_get_expr(c.relpartbound,c.oid),c.relrowsecurity,c.relforcerowsecurity,pg_get_userbyid(c.relowner),c.relacl,c.reloptions,CASE WHEN c.relkind IN ('v','m') THEN pg_get_viewdef(c.oid,true) ELSE '' END)::text
FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations' AND c.relkind IN ('r','p','v','m','S','f')
UNION ALL
SELECT json_build_array('column',c.relname,a.attnum,a.attname,format_type(a.atttypid,a.atttypmod),a.attnotnull,a.attidentity,a.attgenerated,a.attcollation,pg_get_expr(d.adbin,d.adrelid))::text
FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations' AND c.relkind IN ('r','p','v','m','S','f') AND a.attnum>0 AND NOT a.attisdropped
UNION ALL
SELECT json_build_array('constraint',c.relname,x.conname,pg_get_constraintdef(x.oid,true))::text FROM pg_constraint x JOIN pg_class c ON c.oid=x.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations'
UNION ALL
SELECT json_build_array('index',c.relname,pg_get_indexdef(i.indexrelid))::text FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations'
UNION ALL
SELECT json_build_array('policy',c.relname,p.polname,p.polcmd,p.polpermissive,p.polroles,pg_get_expr(p.polqual,p.polrelid),pg_get_expr(p.polwithcheck,p.polrelid))::text FROM pg_policy p JOIN pg_class c ON c.oid=p.polrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations'
UNION ALL
SELECT json_build_array('trigger',c.relname,t.tgname,pg_get_triggerdef(t.oid,true))::text FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname<>'registry_schema_migrations' AND NOT t.tgisinternal
UNION ALL
SELECT json_build_array('function',p.proname,pg_get_function_identity_arguments(p.oid),pg_get_functiondef(p.oid),pg_get_userbyid(p.proowner),p.proacl)::text FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.prokind IN ('f','p')
UNION ALL
SELECT json_build_array('type',t.typname,t.typtype,format_type(t.typbasetype,t.typtypmod),t.typnotnull,t.typdefault,t.typacl)::text FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typrelid=0 AND t.typelem=0
UNION ALL
SELECT json_build_array('enum',t.typname,e.enumlabel,e.enumsortorder)::text FROM pg_enum e JOIN pg_type t ON t.oid=e.enumtypid JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public'
UNION ALL
SELECT json_build_array('domain_constraint',t.typname,x.conname,pg_get_constraintdef(x.oid,true))::text FROM pg_constraint x JOIN pg_type t ON t.oid=x.contypid JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public'
UNION ALL
SELECT json_build_array('sequence',c.relname,s.seqstart,s.seqincrement,s.seqmax,s.seqmin,s.seqcache,s.seqcycle)::text FROM pg_sequence s JOIN pg_class c ON c.oid=s.seqrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
UNION ALL
SELECT json_build_array('extension',e.extname,e.extversion,n.nspname)::text FROM pg_extension e JOIN pg_namespace n ON n.oid=e.extnamespace
UNION ALL
SELECT json_build_array('schema',n.nspname,pg_get_userbyid(n.nspowner),n.nspacl)::text FROM pg_namespace n WHERE n.nspname='public'
`

func schemaFingerprint(ctx context.Context, q queryer) (string, error) {
	rows, err := q.Query(ctx, fingerprintSQL)
	if err != nil {
		return "", errors.New("cannot fingerprint schema definitions")
	}
	var definitions []string
	for rows.Next() {
		var definition string
		if err := rows.Scan(&definition); err != nil {
			rows.Close()
			return "", errors.New("cannot fingerprint schema definitions")
		}
		definitions = append(definitions, definition)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", errors.New("cannot fingerprint schema definitions")
	}
	sort.Strings(definitions)
	raw, _ := json.Marshal(definitions)
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}
