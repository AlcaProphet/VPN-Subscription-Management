package assembly

// render 按目标语法分发渲染。
func (s *Service) render(in GenerateInput, ld *loadedData) (*RenderResult, error) {
	var res *RenderResult
	var err error
	switch in.TargetSyntax {
	case ClashYAML:
		res, err = s.renderClash(in, ld)
	case SrSubs:
		res, err = s.renderSrSubs(in, ld, true)
	case GenericSubs:
		res, err = s.renderSrSubs(in, ld, false)
	case SrConf:
		res, err = s.renderSrConf(in, ld)
	default:
		return nil, ErrBadRequest
	}
	if err != nil {
		return nil, err
	}
	if res != nil {
		res.Receipt = buildReceipt(in, ld)
	}
	return res, nil
}
