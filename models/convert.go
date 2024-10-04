package models

import "gopkg.in/yaml.v3"

// 负责数据模型转换

func RawPipeline2PipelineCfg(raw RawPipeline) (cfg PipelineCfg, err error) {
	cfg.Name = raw.Metadata.Name
	cfg.Version = raw.PipelineVersion
	cfg.PipeType = PipelineType(raw.Metadata.Namespace)
	b, err := yaml.Marshal(raw)
	if err != nil {
		return
	}
	cfg.CfgYaml = string(b)
	return
}
