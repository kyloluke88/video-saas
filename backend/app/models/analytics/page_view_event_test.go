package analytics

import "testing"

func TestNormalizePageType(t *testing.T) {
	// 枚举变更时，这个表驱动测试能第一时间暴露前后端值不一致。
	tests := []struct {
		name       string
		value      int16
		wantType   PageType
		wantExists bool
	}{
		{name: "podcast", value: 1, wantType: PageTypePodcastScriptDetail, wantExists: true},
		{name: "product", value: 2, wantType: PageTypeProductDetail, wantExists: true},
		{name: "collection", value: 3, wantType: PageTypeCollectionPage, wantExists: true},
		{name: "static", value: 4, wantType: PageTypeStaticPage, wantExists: true},
		{name: "invalid", value: 9, wantType: 0, wantExists: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotExists := NormalizePageType(tt.value)
			if gotType != tt.wantType || gotExists != tt.wantExists {
				t.Fatalf("NormalizePageType(%d) = (%v, %v), want (%v, %v)", tt.value, gotType, gotExists, tt.wantType, tt.wantExists)
			}
		})
	}
}

func TestPageTypeRequiresEntityID(t *testing.T) {
	// 详情页必须带 entity id，列表页和静态页则不能强依赖实体。
	if !PageTypePodcastScriptDetail.RequiresEntityID() {
		t.Fatal("podcast detail page should require entity id")
	}
	if !PageTypeProductDetail.RequiresEntityID() {
		t.Fatal("product detail page should require entity id")
	}
	if PageTypeCollectionPage.RequiresEntityID() {
		t.Fatal("collection page should not require entity id")
	}
	if PageTypeStaticPage.RequiresEntityID() {
		t.Fatal("static page should not require entity id")
	}
}
