import { useState } from 'react'
import {
  Container,
  Box,
  Typography,
  Button,
  Card,
  CardContent,
  AppBar,
  Toolbar,
  Stack,
  Avatar,
  Chip,
  Divider,
  IconButton,
  Grid,
  Paper,
  TextField,
  List,
  ListItem,
  ListItemText,
  ListItemIcon,
  Switch,
  FormControlLabel,
  Slider,
  LinearProgress,
  Rating,
  Badge,
  Tooltip,
  Tabs,
  Tab,
  Accordion,
  AccordionSummary,
  AccordionDetails
} from '@mui/material'
import {
  ExpandMore as ExpandMoreIcon,
  Favorite as FavoriteIcon,
  Star as StarIcon,
  Home as HomeIcon,
  Info as InfoIcon,
  Settings as SettingsIcon,
  Send as SendIcon,
  Menu as MenuIcon
} from '@mui/icons-material'

function TabPanel({ children, value, index }) {
  return value === index ? <Box>{children}</Box> : null
}

function App() {
  const [count, setCount] = useState(0)
  const [tabValue, setTabValue] = useState(0)
  const [text, setText] = useState('')
  const [checked, setChecked] = useState(true)
  const [sliderValue, setSliderValue] = useState(50)
  const [rating, setRating] = useState(3)
  const [liked, setLiked] = useState(false)

  const handleTabChange = (event, newValue) => {
    setTabValue(newValue)
  }

  return (
    <Box>
      {/* Header - AppBar instead of nav/header */}
      <AppBar position="static">
        <Toolbar>
          <IconButton edge="start" color="inherit">
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" sx={{ flexGrow: 1 }}>
            GLM React Website
          </Typography>
          <Badge badgeContent={3} color="secondary">
            <IconButton color="inherit">
              <FavoriteIcon />
            </IconButton>
          </Badge>
        </Toolbar>
      </AppBar>

      {/* Navigation Tabs */}
      <Box sx={{ borderBottom: 1, borderColor: 'divider' }}>
        <Tabs value={tabValue} onChange={handleTabChange} centered>
          <Tab icon={<HomeIcon />} label="Home" />
          <Tab icon={<InfoIcon />} label="About" />
          <Tab icon={<SettingsIcon />} label="Settings" />
        </Tabs>
      </Box>

      {/* Main Content */}
      <Container maxWidth="lg" sx={{ mt: 4, mb: 4 }}>
        <TabPanel value={tabValue} index={0}>
          {/* Hero Section */}
          <Paper elevation={3} sx={{ p: 4, mb: 4 }}>
            <Stack direction="column" alignItems="center" spacing={2}>
              <Avatar
                sx={{ width: 80, height: 80, bgcolor: 'primary.main' }}
              >
                GLM
              </Avatar>
              <Typography variant="h3" component="h1" align="center">
                Welcome to GLM
              </Typography>
              <Typography variant="body1" align="center" color="text.secondary">
                A pure React UI website using only React components
              </Typography>
              <Stack direction="row" spacing={2}>
                <Button variant="contained" size="large">
                  Get Started
                </Button>
                <Button variant="outlined" size="large">
                  Learn More
                </Button>
              </Stack>
            </Stack>
          </Paper>

          {/* Features Grid */}
          <Grid container spacing={3} sx={{ mb: 4 }}>
            {[
              { title: 'Pure React', desc: 'No HTML elements used' },
              { title: 'Material UI', desc: 'Beautiful components' },
              { title: 'Fully Interactive', desc: 'State-driven UI' }
            ].map((feature, index) => (
              <Grid item xs={12} md={4} key={index}>
                <Card>
                  <CardContent>
                    <Stack spacing={2}>
                      <Typography variant="h6">
                        {feature.title}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {feature.desc}
                      </Typography>
                    </Stack>
                  </CardContent>
                </Card>
              </Grid>
            ))}
          </Grid>

          {/* Interactive Demo */}
          <Paper elevation={2} sx={{ p: 4 }}>
            <Typography variant="h5" gutterBottom>
              Interactive Demo
            </Typography>
            <Divider sx={{ mb: 3 }} />
            <Stack spacing={3}>
              {/* Counter with Button */}
              <Stack direction="row" alignItems="center" spacing={2}>
                <Button
                  variant="contained"
                  onClick={() => setCount(count + 1)}
                  startIcon={<StarIcon />}
                >
                  Increment
                </Button>
                <Chip label={`Count: ${count}`} color="primary" />
              </Stack>

              {/* Like Button */}
              <Stack direction="row" alignItems="center" spacing={2}>
                <Tooltip title={liked ? 'Unlike' : 'Like'}>
                  <IconButton
                    onClick={() => setLiked(!liked)}
                    color={liked ? 'error' : 'default'}
                  >
                    <FavoriteIcon />
                  </IconButton>
                </Tooltip>
                <Typography variant="body2">
                  {liked ? 'Liked!' : 'Click to like'}
                </Typography>
              </Stack>

              {/* Rating */}
              <Stack direction="row" alignItems="center" spacing={2}>
                <Typography>Rate this:</Typography>
                <Rating
                  value={rating}
                  onChange={(event, newValue) => setRating(newValue)}
                />
              </Stack>

              {/* Progress Bar */}
              <Box>
                <Typography gutterBottom>Progress: {sliderValue}%</Typography>
                <LinearProgress variant="determinate" value={sliderValue} />
              </Box>
            </Stack>
          </Paper>
        </TabPanel>

        <TabPanel value={tabValue} index={1}>
          <Paper elevation={2} sx={{ p: 4 }}>
            <Typography variant="h4" gutterBottom>
              About
            </Typography>
            <Divider sx={{ mb: 3 }} />
            <Stack spacing={2}>
              <Typography variant="body1">
                This website demonstrates building a complete web application using
                only React UI components from Material UI. No HTML elements like
                div, h1, p, button, etc. are used directly.
              </Typography>
              <Typography variant="body1">
                Everything is built using React components such as Box, Typography,
                Card, Button, and more.
              </Typography>

              {/* Accordion FAQ */}
              <Box sx={{ mt: 4 }}>
                <Typography variant="h6" gutterBottom>
                  FAQ
                </Typography>
                <Accordion>
                  <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                    <Typography>Why use only React components?</Typography>
                  </AccordionSummary>
                  <AccordionDetails>
                    <Typography>
                      React components provide better abstraction, theming, and
                      consistency across your application.
                    </Typography>
                  </AccordionDetails>
                </Accordion>
                <Accordion>
                  <AccordionSummary expandIcon={<ExpandMoreIcon />}>
                    <Typography>What library is being used?</Typography>
                  </AccordionSummary>
                  <AccordionDetails>
                    <Typography>
                      Material UI (MUI) - a popular React component library.
                    </Typography>
                  </AccordionDetails>
                </Accordion>
              </Box>
            </Stack>
          </Paper>
        </TabPanel>

        <TabPanel value={tabValue} index={2}>
          <Paper elevation={2} sx={{ p: 4 }}>
            <Typography variant="h4" gutterBottom>
              Settings
            </Typography>
            <Divider sx={{ mb: 3 }} />
            <Stack spacing={3}>
              {/* Text Input */}
              <TextField
                fullWidth
                label="Your Name"
                value={text}
                onChange={(e) => setText(e.target.value)}
                variant="outlined"
                helperText="Enter your name"
              />

              {/* Switch Toggle */}
              <FormControlLabel
                control={
                  <Switch
                    checked={checked}
                    onChange={(e) => setChecked(e.target.checked)}
                  />
                }
                label="Enable notifications"
              />

              {/* Slider */}
              <Box>
                <Typography gutterBottom>Volume</Typography>
                <Slider
                  value={sliderValue}
                  onChange={(e, newValue) => setSliderValue(newValue)}
                  valueLabelDisplay="auto"
                />
              </Box>

              {/* List Items */}
              <List>
                <ListItem>
                  <ListItemIcon>
                    <HomeIcon />
                  </ListItemIcon>
                  <ListItemText primary="Home Page" secondary="Landing page" />
                </ListItem>
                <ListItem>
                  <ListItemIcon>
                    <InfoIcon />
                  </ListItemIcon>
                  <ListItemText primary="About" secondary="Information page" />
                </ListItem>
                <ListItem>
                  <ListItemIcon>
                    <SettingsIcon />
                  </ListItemIcon>
                  <ListItemText primary="Settings" secondary="Configuration" />
                </ListItem>
              </List>

              {/* Action Buttons */}
              <Stack direction="row" spacing={2}>
                <Button variant="contained" endIcon={<SendIcon />}>
                  Save
                </Button>
                <Button variant="outlined" color="error">
                  Cancel
                </Button>
              </Stack>
            </Stack>
          </Paper>
        </TabPanel>
      </Container>

      {/* Footer */}
      <Paper sx={{ mt: 'auto', py: 3 }} component="footer">
        <Container maxWidth="lg">
          <Stack direction="row" justifyContent="space-between" alignItems="center">
            <Typography variant="body2" color="text.secondary">
              © 2024 GLM React Website
            </Typography>
            <Stack direction="row" spacing={2}>
              <Chip label="React" size="small" />
              <Chip label="Material UI" size="small" />
              <Chip label="Vite" size="small" />
            </Stack>
          </Stack>
        </Container>
      </Paper>
    </Box>
  )
}

export default App
