package mavendeps

import "testing"

func TestParsePOM_ignoresCommentedDependencies(t *testing.T) {
	const perLine = `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-conversion-edi</artifactId>
<version>1.0</version>
<dependencies>
<!--      <dependency>-->
<!--          <groupId>com.flux.wmsv6</groupId>-->
<!--          <artifactId>flux-collaboration-storage-adapter</artifactId>-->
<!--          <version>${datahub.core.version}</version>-->
<!--      </dependency>-->
</dependencies>
</project>`
	p, err := ParsePOM(perLine)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Dependencies) != 0 {
		t.Fatalf("per-line comment deps=%d want 0: %+v", len(p.Dependencies), p.Dependencies)
	}

	const block = `<project>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-conversion-edi</artifactId>
<version>1.0</version>
<dependencies>
<!--
<dependency>
<groupId>com.flux.wmsv6</groupId>
<artifactId>flux-collaboration-storage-adapter</artifactId>
<version>1.0</version>
</dependency>
-->
</dependencies>
</project>`
	p2, err := ParsePOM(block)
	if err != nil {
		t.Fatal(err)
	}
	if len(p2.Dependencies) != 0 {
		t.Fatalf("block comment deps=%d want 0: %+v", len(p2.Dependencies), p2.Dependencies)
	}
}
