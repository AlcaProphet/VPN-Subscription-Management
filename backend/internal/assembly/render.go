package assembly

// render 按目标语法分发渲染。
func (s *Service) render(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	switch in.TargetSyntax {
	case ClashYAML:
		return s.renderClash(in, ld)
	case SrSubs:
		return s.renderSrSubs(in, ld, true)
	case GenericSubs:
		return s.renderSrSubs(in, ld, false)
	case SrConf:
		return s.renderSrConf(in, ld)
	default:
		return nil, ErrBadRequest
	}
}
