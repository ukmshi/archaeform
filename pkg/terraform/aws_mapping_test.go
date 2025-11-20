package terraform

import "testing"

func TestAwsToResourceMapper_MapSubnet(t *testing.T) {
	mapper := NewAwsToResourceMapper(NewDefaultNameGenerator())

	subnets := []RawSubnet{
		{
			ID:        "subnet-1234",
			VpcID:     "vpc-1111",
			CidrBlock: "10.0.1.0/24",
			Name:      "subnet-public-a",
			Tags:      map[string]string{"Name": "public-a", "Env": "prod"},
			Az:        "ap-northeast-1a",
		},
	}

	resources, relations, err := mapper.MapSubnet(subnets, "ap-northeast-1")
	if err != nil {
		t.Fatalf("MapSubnet returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if len(relations) != 1 {
		t.Fatalf("expected 1 relation, got %d", len(relations))
	}

	r := resources[0]
	if r.Type != "aws_subnet" || r.Provider != "aws" {
		t.Fatalf("unexpected resource type/provider: %q/%q", r.Type, r.Provider)
	}
	if r.Labels["aws_region"] != "ap-northeast-1" {
		t.Fatalf("expected aws_region label to be set")
	}
	if r.Attributes["cidr_block"] != "10.0.1.0/24" {
		t.Fatalf("expected cidr_block attribute to be set")
	}

	rel := relations[0]
	if rel.Kind != RelationNetwork {
		t.Fatalf("expected relation kind %q, got %q", RelationNetwork, rel.Kind)
	}
	if rel.From != r.ID {
		t.Fatalf("expected relation From = resource ID")
	}
}

func TestAwsToResourceMapper_MapInstance(t *testing.T) {
	mapper := NewAwsToResourceMapper(NewDefaultNameGenerator())

	instances := []RawInstance{
		{
			ID:               "i-1234",
			Ami:              "ami-aaaa",
			InstanceType:     "t3.micro",
			SubnetID:         "subnet-1234",
			SecurityGroupIDs: []string{"sg-1", "sg-2"},
			Tags:             map[string]string{"Name": "web-1"},
		},
	}

	resources, relations, err := mapper.MapInstance(instances, "ap-northeast-1")
	if err != nil {
		t.Fatalf("MapInstance returned error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	if len(relations) != 3 { // 1 subnet + 2 SG
		t.Fatalf("expected 3 relations, got %d", len(relations))
	}

	r := resources[0]
	if r.Type != "aws_instance" {
		t.Fatalf("unexpected resource type: %q", r.Type)
	}
	if r.Attributes["instance_type"] != "t3.micro" {
		t.Fatalf("expected instance_type attribute to be set")
	}
	if r.Attributes["subnet_id"] != "subnet-1234" {
		t.Fatalf("expected subnet_id attribute to be raw subnet ID before HCL generation")
	}
}


